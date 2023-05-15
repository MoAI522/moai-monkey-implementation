package evaluator

import (
	"fmt"
	"monkey/ast"
	"monkey/lexer"
	"monkey/object"
	"monkey/parser"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	// REVIEW[FLOW]: ここも長いswitch文だが、各ケースの処理が関数に切り出されている&関数名がわかりやすいので全体的に読みやすい。
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		return evalProgram(node.Statements, env, threadPool)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env, threadPool)
	case *ast.BlockStatement:
		return evalBlockStatements(node, env, threadPool)
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env, threadPool)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		val := Eval(node.Value, env, threadPool)
		if isError(val) {
			return val
		}
		return env.Set(node.Name.Value, val) // アレンジ、let文は値を返す
	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.PrefixExpression:
		right := Eval(node.Right, env, threadPool)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := Eval(node.Left, env, threadPool)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env, threadPool)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IfExpression:
		return evalIfExpression(node, env, threadPool)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}
	case *ast.CallExpression:
		if node.Function.TokenLiteral() == "quote" {
			return quote(node.Arguments[0], env, threadPool)
		}
		function := Eval(node.Function, env, threadPool)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env, threadPool)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args, env, threadPool)
	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env, threadPool)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.IndexExpression:
		left := Eval(node.Left, env, threadPool)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env, threadPool)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	case *ast.HashLiteral:
		return evalHashLiteral(node, env, threadPool)
	}
	return nil
}

func evalProgram(stmts []ast.Statement, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	var result object.Object
	for _, statement := range stmts {
		result = Eval(statement, env, threadPool)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
		// REVIEW: これたぶん消し忘れ
		if returnValue, ok := result.(*object.ReturnValue); ok {
			return returnValue.Value
		}
	}
	return result
}

func evalBlockStatements(block *ast.BlockStatement, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	var result object.Object
	for _, statement := range block.Statements {
		result = Eval(statement, env, threadPool)
		if result != nil {
			rt := result.Type()
			// REVIEW[FLOW]: 変数が左、定数が右
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}
	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

// REVIEW[NAMING]: eval〇〇(Node種類)という命名はフォーマットができている。(2.6 名前のフォーマットで情報を伝える)
// それぞれの機能が同列であることが分かり読みやすい。フォーマットがないと、引数はバラバラであるため、同じくくりに見えない。
func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinuxPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		// TODO: 0もtrue扱いしてるのでfalse扱いに
		return FALSE
	}
}

func evalMinuxPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER_OBJ {
		return newError("unknown operator: -%s", right.Type())
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)

	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value
	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	// REVIEW[SPLIT]: 説明変数?
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	condition := Eval(ie.Condition, env, threadPool)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env, threadPool)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env, threadPool)
	} else {
		return NULL
	}
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	switch node.Value {
	case "eval":
		return &object.BuiltinEval{}
	case "launch":
		return &object.BuiltinLaunch{}
	case "await":
		return &object.BuiltinAwait{}
	}

	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := buildins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: " + node.Value)
}

func evalExpressions(
	exps []ast.Expression,
	env *object.Environment,
	threadPool *object.ThreadPool,
) []object.Object {
	var result []object.Object
	for _, e := range exps {
		evaluated := Eval(e, env, threadPool)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}
	return result
}

func applyFunction(fn object.Object, args []object.Object, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv, threadPool)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		return fn.Fn(args...)
	case *object.BuiltinEval:
		return builtinEvalFunction(args[0], env, threadPool)
	case *object.BuiltinLaunch:
		return builtinLaunchFunction(args[0], env, threadPool)
	case *object.BuiltinAwait:
		return builtinAwaitFunction(args[0], threadPool)
	default:
		return newError("not a function: %s", fn.Type())
	}

}

// REVIEW[SPLIT]: 下位問題の抽出
func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}
	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)
	if idx < 0 || idx > max {
		return NULL
	}
	return arrayObject.Elements[idx]
}

func evalHashLiteral(node *ast.HashLiteral, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)
	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env, threadPool)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("unusuable as hash key: %s", key.Type())
		}
		value := Eval(valueNode, env, threadPool)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}
	return &object.Hash{Pairs: pairs}
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)
	key, ok := index.(object.Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}
	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}
	return pair.Value
}

func builtinEvalFunction(obj object.Object, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	if obj.Type() != object.STRING_OBJ {
		return newError("argument of eval must be string. got=%T", obj.Type())
	}
	str, _ := obj.(*object.String)
	l := lexer.New(str.Value)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return newError("parse error at `eval`: %q", p.Errors())
	}
	extendedenv := object.NewEnclosedEnvironment(env)
	return Eval(program, extendedenv, threadPool)
}

func builtinLaunchFunction(obj object.Object, env *object.Environment, threadPool *object.ThreadPool) object.Object {
	if obj.Type() != object.FUNCTION_OBJ {
		return newError("argument for launch must be FUNCTION. got=%T", obj.Type())
	}

	fn, _ := obj.(*object.Function)
	extendedEnv := object.NewEnclosedEnvironment(env)
	c := make(chan object.Object, 2)
	go miniRoutine(c, fn, extendedEnv, threadPool)
	threadID := threadPool.Set(c)

	return &object.ThreadID{Value: threadID}
}

func miniRoutine(c chan object.Object, fn *object.Function, env *object.Environment, threadPool *object.ThreadPool) {
	c <- Eval(fn.Body, env, threadPool)
	close(c)
}

// REVIEW[NAMING, COMMENT]: objという引数だと、何が入るのか分かりづらいが、型チェックをしていないので何が入っているか分からないという内部処理的な意味もあり、悩ましい
func builtinAwaitFunction(obj object.Object, threadPool *object.ThreadPool) object.Object {
	// REVIEW[FLOW]: early return
	if obj.Type() != object.THREAD_ID_OBJ {
		return newError("argument for await must be THREAD_ID. got=%T", obj.Type())
	}
	threadID, _ := obj.(*object.ThreadID)
	c, ok := threadPool.Get(threadID.Value)
	if !ok {
		return newError("thread not found: %q", threadID)
	}

	// REVIEW[COMMENT]: Javaでいうところのjoin処理という意味でコメントしたが、分かりづらいかも
	// join thread
	tmp, ok := <-c
	var result object.Object
	result = NULL
	for ok {
		result = tmp
		tmp, ok = <-c
	}

	return result
}
