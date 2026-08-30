package objects

import (
	"strconv"

	"github.com/innovationreadytupperware/danser-ee/app/settings"
)

func CreateObject(data []string) (object IHitObject) {
	// User-authored .osu files can contain truncated object rows. Object
	// constructors historically indexed their format fields directly, so keep
	// malformed rows from turning a lazy map load into a process-wide panic.
	defer func() {
		if recover() != nil {
			object = nil
		}
	}()

	if len(data) < 4 {
		return nil
	}

	objTypeI, _ := strconv.Atoi(data[3])
	objType := Type(objTypeI)

	if (objType & CIRCLE) > 0 {
		if len(data) < 5 {
			return nil
		}
		return NewCircle(data)
	} else if (objType & SPINNER) > 0 {
		if len(data) < 6 {
			return nil
		}
		if settings.Objects.LoadSpinners || settings.KNOCKOUT || settings.PLAY {
			return NewSpinner(data)
		}
	} else if (objType & SLIDER) > 0 {
		if len(data) < 8 {
			return nil
		}
		if sl := NewSlider(data); sl != nil {
			return sl
		}
	}

	return nil
}

type Type int

const (
	CIRCLE = Type(1 << iota)
	SLIDER
	NEWCOMBO
	SPINNER
	LONGNOTE = Type(128) //only for mania, used to have correct number of sliders in database just in case
)
