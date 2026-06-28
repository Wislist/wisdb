package client

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type Shell interface {
	Run()
	SetDisconnCh(ch <-chan struct{})
}

type shell struct {
	client    Client
	reconnect func() (Client, <-chan struct{})
	disconnCh <-chan struct{}
}

func NewShell(client Client) *shell {
	return &shell{client: client}
}

func NewShellWithReconnect(client Client, reconnect func() (Client, <-chan struct{})) *shell {
	return &shell{client: client, reconnect: reconnect}
}

func (s *shell) SetDisconnCh(ch <-chan struct{}) {
	s.disconnCh = ch
}

func (s *shell) Run() {
	defer s.client.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		s.runSimple()
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var history [][]byte
	histIdx := -1

	buf := make([]byte, 0, 256)
	cursor := 0

	prompt := ":> "
	fmt.Print(prompt)

	type byteResult struct {
		b   byte
		err error
	}
	byteCh := make(chan byteResult, 1)

	startReader := func() {
		go func() {
			b := make([]byte, 1)
			_, err := os.Stdin.Read(b)
			byteCh <- byteResult{b[0], err}
		}()
	}

	redraw := func() {
		fmt.Printf("\r\033[K%s%s", prompt, string(buf))
		if cursor < len(buf) {
			fmt.Printf("\033[%dD", len(buf)-cursor)
		}
	}

	handleDisconnect := func() {
		if s.reconnect == nil {
			return
		}
		fmt.Printf("\r\n\033[K[Server disconnected. Reconnecting...]\r\n")
		s.client, s.disconnCh = s.reconnect()
		redraw()
	}

	startReader()

	for {
		select {
		case br := <-byteCh:
			if br.err != nil {
				fmt.Print("\r\n")
				return
			}
			b := br.b

			switch b {
			case 3: // Ctrl+C
				fmt.Print("\r\n")
				return

			case 4: // Ctrl+D
				fmt.Print("\r\n")
				return

			case 13, 10: // Enter
				fmt.Print("\r\n")
				line := string(buf)

				if line == "exit" || line == "quit" {
					return
				}
				if line == "clear" {
					fmt.Print("\033[2J\033[H")
					buf = buf[:0]
					cursor = 0
					histIdx = -1
					fmt.Print(prompt)
					startReader()
					continue
				}

				if len(buf) > 0 {
					saved := make([]byte, len(buf))
					copy(saved, buf)
					history = append(history, saved)
				}
				histIdx = -1

				if len(buf) > 0 {
					result, err := s.client.Execute(buf)
					if err != nil {
						if !s.client.IsConnected() && s.reconnect != nil {
							fmt.Printf("\033[K[Server disconnected. Reconnecting...]\r\n")
							s.client, s.disconnCh = s.reconnect()
							result, err = s.client.Execute(buf)
							if err != nil {
								fmt.Printf("Err:  %v\r\n", err)
							} else {
								out := strings.ReplaceAll(string(result), "\n", "\r\n")
								fmt.Printf("%s\r\n", out)
							}
						} else {
							fmt.Printf("Err:  %v\r\n", err)
						}
					} else {
						out := strings.ReplaceAll(string(result), "\n", "\r\n")
						fmt.Printf("%s\r\n", out)
					}
				}

				buf = buf[:0]
				cursor = 0
				fmt.Print(prompt)

			case 127, 8: // Backspace
				if cursor > 0 {
					buf = append(buf[:cursor-1], buf[cursor:]...)
					cursor--
					redraw()
				}

			case 27: // ESC sequence
				// 需要再读 2 字节，直接同步读（已在 raw 模式，很快返回）
				startReader()
				br2 := <-byteCh
				if br2.b != '[' {
					startReader()
					continue
				}
				startReader()
				br3 := <-byteCh
				switch br3.b {
				case 'A': // Up
					if len(history) == 0 {
						startReader()
						continue
					}
					if histIdx == -1 {
						histIdx = len(history) - 1
					} else if histIdx > 0 {
						histIdx--
					}
					buf = make([]byte, len(history[histIdx]))
					copy(buf, history[histIdx])
					cursor = len(buf)
					redraw()
				case 'B': // Down
					if histIdx == -1 {
						startReader()
						continue
					}
					histIdx++
					if histIdx >= len(history) {
						histIdx = -1
						buf = buf[:0]
						cursor = 0
					} else {
						buf = make([]byte, len(history[histIdx]))
						copy(buf, history[histIdx])
						cursor = len(buf)
					}
					redraw()
				case 'C': // Right
					if cursor < len(buf) {
						cursor++
						fmt.Print("\033[1C")
					}
				case 'D': // Left
					if cursor > 0 {
						cursor--
						fmt.Print("\033[1D")
					}
				}

			default:
				if b >= 32 && b < 127 {
					buf = append(buf, 0)
					copy(buf[cursor+1:], buf[cursor:])
					buf[cursor] = b
					cursor++
					redraw()
				}
			}

			startReader()

		case <-s.disconnCh:
			handleDisconnect()
		}
	}
}

// runSimple 非交互终端（如管道）时的降级模式
func (s *shell) runSimple() {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1)
	fmt.Print(":> ")
	for {
		_, err := os.Stdin.Read(tmp)
		if err != nil {
			break
		}
		if tmp[0] == '\n' {
			line := string(buf)
			buf = buf[:0]
			if line == "exit" || line == "quit" {
				return
			}
			result, err := s.client.Execute([]byte(line))
			if err != nil {
				if !s.client.IsConnected() && s.reconnect != nil {
					fmt.Println("[Server disconnected. Reconnecting...]")
					s.client, s.disconnCh = s.reconnect()
					result, err = s.client.Execute([]byte(line))
					if err != nil {
						fmt.Println("Err: ", err)
					} else {
						fmt.Println(string(result))
					}
				} else {
					fmt.Println("Err: ", err)
				}
			} else {
				fmt.Println(string(result))
			}
			fmt.Print(":> ")
		} else {
			buf = append(buf, tmp[0])
		}
	}
}
