package repl

import (
	"bufio"
	"fmt"
	"io"
	"monkey/lexer"
	"monkey/parser"
)

const PROMPT = ">> "
const OUR_FACE = `                ＿＿＿_     
            ／_ノ    ヽ､_＼
  ﾐ ﾐ ﾐ    oﾟ(（●）) (（●）)ﾟo      ﾐ ﾐ ﾐ
/⌒)⌒)⌒.   ::::⌒（__人__）⌒::＼    /⌒)⌒)⌒)
| / / /           |r┬-|      |(⌒)/ / / /／
|  :::::(⌒)       | | |     ／  ゝ   ::::/                                 
|        ノ       | | |     ＼    /  ） /
ヽ       /         'ー'´       ヽ /     ／  バ
 |      |   |从人|               |从人||   ン
 ヽ        -''"~~'\'-､       -一'''''''--､  バ
  ヽ ＿＿＿(⌒)(⌒)⌒) )         (⌒＿(⌒)⌒)⌒)) ン
`

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)

	for {
		fmt.Printf(PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParseErrors(out, p.Errors())
			continue
		}

		io.WriteString(out, program.String())
		io.WriteString(out, "\n")
	}
}

func printParseErrors(out io.Writer, errors []string) {
	io.WriteString(out, OUR_FACE)
	io.WriteString(out, "parse errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
