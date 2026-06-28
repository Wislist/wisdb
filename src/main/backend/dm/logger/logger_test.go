package logger

import "testing"

func TestLogger(t *testing.T) {
	lg, _ := Create("test_logger")
	lg.Log([]byte("aaa"))
	lg.Log([]byte("bbb"))
	lg.Log([]byte("ccc"))
	lg.Log([]byte("ddd"))
	lg.Log([]byte("eee"))
	lg.Close()

	lg, _ = Open("test_logger")
	lg.Rewind()

	log, ok, _ := lg.Next()
	if ok == false {
		t.Fatal("error")
	}
	if string(log) != string("aaa") {
		t.Fatal("error")
	}

	log, ok, _ = lg.Next()
	if ok == false {
		t.Fatal("error")
	}
	if string(log) != string("bbb") {
		t.Fatal("error")
	}

	log, ok, _ = lg.Next()
	if ok == false {
		t.Fatal("error")
	}
	if string(log) != string("ccc") {
		t.Fatal("error")
	}

	log, ok, _ = lg.Next()
	if ok == false {
		t.Fatal("error")
	}
	if string(log) != string("ddd") {
		t.Fatal("error")
	}

	log, ok, _ = lg.Next()
	if ok == false {
		t.Fatal("error")
	}
	if string(log) != string("eee") {
		t.Fatal("error")
	}

	_, ok, _ = lg.Next()
	if ok != false {
		t.Fatal("error")
	}

}
