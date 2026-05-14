// Package wisdb provides a Go driver for WisDB.
//
// Quick start:
//
//	pool, err := wisdb.NewPool("localhost:8080", wisdb.DefaultPoolOptions())
//	if err != nil { ... }
//	defer pool.Close()
//
//	conn, err := pool.Get()
//	if err != nil { ... }
//	defer pool.Put(conn)
//
//	rows, err := conn.Execute("read * from user where id = 1")
//	// or use a transaction:
//	tx, err := conn.Begin(wisdb.ReadCommitted)
//	tx.Execute("insert into user values 2 'bob' 25")
//	tx.Commit()
package wisdb
