package client

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type Shell interface {
	Run()
}

type shell struct {
	client Client
}

func NewShell(client Client) *shell {
	return &shell{client: client}
}

func (s *shell) Run() {
	defer s.client.Close()

	// 切换终端到 raw 模式，使方向键等控制字符可被逐字节读取
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// 无法切换 raw 模式（如非交互终端），退回简单模式
		s.runSimple()
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var history [][]byte // 历史记录
	histIdx := -1        // 当前浏览的历史位置，-1 表示当前输入行

	buf := make([]byte, 0, 256) // 当前输入缓冲
	cursor := 0                 // 光标在 buf 中的位置

	prompt := ":> "
	fmt.Print(prompt)

	readByte := func() (byte, error) {
		b := make([]byte, 1)
		_, err := os.Stdin.Read(b)
		return b[0], err
	}

	// 重新渲染当前行
	redraw := func() {
		// 回到行首，清除整行，重新打印
		fmt.Printf("\r\033[K%s%s", prompt, string(buf))
		// 将光标移动到正确位置
		if cursor < len(buf) {
			fmt.Printf("\033[%dD", len(buf)-cursor)
		}
	}

	for {
		b, err := readByte()
		if err != nil {
			fmt.Print("\r\n")
			break
		}

		switch b {
		case 3: // Ctrl+C
			fmt.Print("\r\n")
			return

		case 4: // Ctrl+D / EOF
			fmt.Print("\r\n")
			return

		case 13, 10: // Enter（\r 或 \n）
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
					fmt.Printf("Err:  %v\r\n", err)
				} else {
					fmt.Printf("%s\r\n", string(result))
				}
			}

			buf = buf[:0]
			cursor = 0
			fmt.Print(prompt)

		case 127, 8: // Backspace / Delete
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
				redraw()
			}

		case 27: // ESC 序列（方向键）
			b2, _ := readByte()
			if b2 != '[' {
				continue
			}
			b3, _ := readByte()
			switch b3 {
			case 'A': // 上方向键：上一条历史
				if len(history) == 0 {
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

			case 'B': // 下方向键：下一条历史
				if histIdx == -1 {
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

			case 'C': // 右方向键：光标右移
				if cursor < len(buf) {
					cursor++
					fmt.Print("\033[1C")
				}

			case 'D': // 左方向键：光标左移
				if cursor > 0 {
					cursor--
					fmt.Print("\033[1D")
				}
			}

		default:
			if b >= 32 && b < 127 { // 可打印 ASCII
				// 在光标位置插入字符
				buf = append(buf, 0)
				copy(buf[cursor+1:], buf[cursor:])
				buf[cursor] = b
				cursor++
				redraw()
			}
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
				fmt.Println("Err: ", err)
			} else {
				fmt.Println(string(result))
			}
			fmt.Print(":> ")
		} else {
			buf = append(buf, tmp[0])
		}
	}
}
