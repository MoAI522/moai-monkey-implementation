package object

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

// REVIEW[NAMING]: Getという名前だが、計算量が微妙に定数時間でない？
// インターフェイスとして、Get-Setが対になっていることの方が重要　中身を知らなくても使える
// 誰向けのプログラムかによって変わる　機能を使う側が主か、機能を提供する側か
// 使う側が主なら、最適化を頑張るきっかけにもなる
// あまりにもどうしようもなく重いならそういう名前にしても
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}
