// An Example with an Output expectation needs the testing package's
// temp-file stdout capture, which wasmbrowsertest's in-browser filesystem
// shim cannot provide (observed: open go-example-stdout-*.txt fails).
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package flow

import "fmt"

func Example() {
	name := New("World")
	count := New(0)

	greeting := Derive(func() string {
		return fmt.Sprintf("Hello %s (%d)", name.Get(), count.Get())
	}, name, count)

	fmt.Println(greeting.Get())

	name.Set("Bellui")
	fmt.Println(greeting.Get())

	count.Set(42)
	fmt.Println(greeting.Get())

	// Output:
	// Hello World (0)
	// Hello Bellui (0)
	// Hello Bellui (42)
}
