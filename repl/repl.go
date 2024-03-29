package repl

import (
	"bufio"
	"fmt"
	"io"
	"monkey/evaluator"
	"monkey/lexer"
	"monkey/object"
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

// REVIEW[COMMENT]: 全体的な処理のフローがここにまとまっているので、動作原理を知っていれば目当ての処理がどこにあるのかわかりやすい
// 一方で、インタプリタの知識が無いと何のためのコードなのかわからないのではないか？
// ここの各処理にはコメントつけた方が、ドメイン知識なくても分かりやすくなる？
func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := object.NewEnvironment()
	macroEnv := object.NewEnvironment()
	threadPool := object.NewThreadPool()

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

		evaluator.DefineMacros(program, macroEnv)
		expanded := evaluator.ExpandMacros(program, macroEnv, threadPool)

		evaluated := evaluator.Eval(expanded, env, threadPool)
		if evaluated != nil {
			io.WriteString(out, evaluated.Inspect())
			io.WriteString(out, "\n")
		}
	}
}

func printParseErrors(out io.Writer, errors []string) {
	io.WriteString(out, OUR_FACE)
	io.WriteString(out, "parse errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
