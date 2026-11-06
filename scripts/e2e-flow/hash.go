// Prints a bcrypt hash for the demo operator account. Run inside the backend
// module so the pinned golang.org/x/crypto version is used.
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(hash))
}
