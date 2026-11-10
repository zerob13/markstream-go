package markstream_test

import (
	"fmt"
	markstream "github.com/zerob13/markstream-go"
)

func ExampleSession() {
	reply := markstream.NewSession(1024)
	_ = reply.Append("Hello **世界**")
	_ = reply.Finish()
	snapshot := reply.Snapshot()
	fmt.Println(snapshot.Source)
	fmt.Println(snapshot.State)
	// Output:
	// Hello **世界**
	// completed
}
func ExampleNewDocument() {
	doc := markstream.NewDocument(0)
	doc.SetStreaming(true)
	doc.Append("Hello **wor")
	doc.Append("ld**")
	doc.SetStreaming(false)
	fmt.Println(doc.Source())
	// Output: Hello **world**
}
