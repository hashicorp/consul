// +build koala_go

package witch

import (
	"runtime"
)

func setCurrentGoRoutineIsKoala() {
	runtime.SetCurrentGoRoutineIsKoala()
}
