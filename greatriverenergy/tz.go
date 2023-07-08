package greatriverenergy

import (
	"time"
	_ "time/tzdata"
)

// "CPT" seems to mean "Central Prevailing Time", i.e. "time in Chicago"
// Get this ready as a time.Location
var tz *time.Location

func init() {
	var err error
	tz, err = time.LoadLocation("America/Chicago")
	if err != nil {
		panic(err)
	}
}
