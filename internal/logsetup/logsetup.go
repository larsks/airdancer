package logsetup

import "log"

func init() {
	log.SetFlags(log.Lshortfile)
}
