package client

/*
   client实现了客户端的API和命令行UI

   大概结构为:
        [shell]       [user process]
            |              |
            v              |
        [client] <---------+
            |
            v
      [RoundTripper]

    shell为用户提供了一个简单的命令行形式的UI.
    当然你也可以不用shell, 自己编写程序, 然后调用client作为访问数据库的API.
    client将需要数据库执行的指令打包, 并传递给RoundTripper.
    RoundTripper进行一次包的"发送->接受"工作.
    RoundTripper依赖于transporter包.
*/


import (
	"mydb/src/main/transporter"
)

type Client interface {
	Execute(stat []byte) ([]byte, error)
	Close()
	Reconnect(pkger transporter.Packager)
	IsConnected() bool
	Disconnected() <-chan struct{}
}

type client struct {
	roundTripper RoundTripper
	closed       bool
}

func NewClient(packager transporter.Packager) *client {
	return &client{
		roundTripper: NewRoundTripper(packager),
	}
}

func (c *client) Close() {
	if !c.closed {
		c.roundTripper.Close()
		c.closed = true
	}
}

func (c *client) Reconnect(pkger transporter.Packager) {
	c.roundTripper = NewRoundTripper(pkger)
	c.closed = false
}

func (c *client) IsConnected() bool {
	if c.closed {
		return false
	}
	select {
	case <-c.roundTripper.Disconnected():
		return false
	default:
		return true
	}
}

func (c *client) Disconnected() <-chan struct{} {
	return c.roundTripper.Disconnected()
}

func (c *client) Execute(stat []byte) ([]byte, error) {
	statPkg := transporter.NewPackage(stat, nil)
	pkg, err := c.roundTripper.RoundTrip(statPkg)
	if err != nil {
		return nil, err
	}
	if pkg.Err() != nil {
		return nil, pkg.Err()
	}
	return pkg.Data(), nil
}
