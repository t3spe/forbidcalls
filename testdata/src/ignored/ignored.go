package ignored

import "os"

func F() {
	_ = os.Getenv("X") //forbidcalls:ignore -- inline suppression
	_ = os.Getenv("Y") // want `forbidden reference to os\.Getenv`

	//forbidcalls:ignore -- leading-comment suppression
	_ = os.Getenv("Z")

	_ = os.Getenv("W") // want `forbidden reference to os\.Getenv`
}
