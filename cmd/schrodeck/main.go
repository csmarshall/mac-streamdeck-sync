// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command schrodeck keeps Stream Deck setups identical across computers.
package main

import (
	"fmt"

	"github.com/csmarshall/schrodeck/internal/version"
)

func main() {
	fmt.Println("schrodeck", version.Version)
}
