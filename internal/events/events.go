package events

import (
	"fmt"
	"syscall"
)

// Input event structure matching the Linux kernel's input_event
type InputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type EventType uint16

// Event types (from linux/input.h)
const (
	EV_SYN       EventType = 0x00
	EV_KEY       EventType = 0x01
	EV_REL       EventType = 0x02
	EV_ABS       EventType = 0x03
	EV_MSC       EventType = 0x04
	EV_SW        EventType = 0x05
	EV_LED       EventType = 0x11
	EV_SND       EventType = 0x12
	EV_REP       EventType = 0x14
	EV_FF        EventType = 0x15
	EV_PWR       EventType = 0x16
	EV_FF_STATUS EventType = 0x17
)

// Key states
const (
	KEY_RELEASED = 0
	KEY_PRESSED  = 1
	KEY_REPEATED = 2
)

// Common key codes (subset)
var keyCodes = map[uint16]string{
	1:   "ESC",
	2:   "1",
	3:   "2",
	4:   "3",
	5:   "4",
	6:   "5",
	7:   "6",
	8:   "7",
	9:   "8",
	10:  "9",
	11:  "0",
	12:  "MINUS",
	13:  "EQUAL",
	14:  "BACKSPACE",
	15:  "TAB",
	16:  "Q",
	17:  "W",
	18:  "E",
	19:  "R",
	20:  "T",
	21:  "Y",
	22:  "U",
	23:  "I",
	24:  "O",
	25:  "P",
	26:  "LEFTBRACE",
	27:  "RIGHTBRACE",
	28:  "ENTER",
	29:  "LEFTCTRL",
	30:  "A",
	31:  "S",
	32:  "D",
	33:  "F",
	34:  "G",
	35:  "H",
	36:  "J",
	37:  "K",
	38:  "L",
	39:  "SEMICOLON",
	40:  "APOSTROPHE",
	41:  "GRAVE",
	42:  "LEFTSHIFT",
	43:  "BACKSLASH",
	44:  "Z",
	45:  "X",
	46:  "C",
	47:  "V",
	48:  "B",
	49:  "N",
	50:  "M",
	51:  "COMMA",
	52:  "DOT",
	53:  "SLASH",
	54:  "RIGHTSHIFT",
	55:  "KPASTERISK",
	56:  "LEFTALT",
	57:  "SPACE",
	58:  "CAPSLOCK",
	103: "UP",
	105: "LEFT",
	106: "RIGHT",
	108: "DOWN",
	272: "BTN_LEFT",
	273: "BTN_RIGHT",
	274: "BTN_MIDDLE",
}

// Relative axis codes
var RelCodes = map[uint16]string{
	0:  "X",
	1:  "Y",
	2:  "Z",
	6:  "HWHEEL",
	8:  "WHEEL",
	9:  "MISC",
	10: "RESERVED",
	11: "WHEEL_HI_RES",
	12: "HWHEEL_HI_RES",
}

// Absolute axis codes
var AbsCodes = map[uint16]string{
	0:  "X",
	1:  "Y",
	2:  "Z",
	3:  "RX",
	4:  "RY",
	5:  "RZ",
	6:  "THROTTLE",
	7:  "RUDDER",
	8:  "WHEEL",
	9:  "GAS",
	10: "BRAKE",
	16: "HAT0X",
	17: "HAT0Y",
	18: "HAT1X",
	19: "HAT1Y",
	20: "HAT2X",
	21: "HAT2Y",
	22: "HAT3X",
	23: "HAT3Y",
	24: "PRESSURE",
	25: "DISTANCE",
	26: "TILT_X",
	27: "TILT_Y",
	28: "TOOL_WIDTH",
	32: "VOLUME",
	40: "MISC",
	47: "MT_SLOT",
	48: "MT_TOUCH_MAJOR",
	49: "MT_TOUCH_MINOR",
	50: "MT_WIDTH_MAJOR",
	51: "MT_WIDTH_MINOR",
	52: "MT_ORIENTATION",
	53: "MT_POSITION_X",
	54: "MT_POSITION_Y",
	55: "MT_TOOL_TYPE",
	56: "MT_BLOB_ID",
	57: "MT_TRACKING_ID",
	58: "MT_PRESSURE",
	59: "MT_DISTANCE",
	60: "MT_TOOL_X",
	61: "MT_TOOL_Y",
}

func GetEventTypeCode(eventType EventType) string {
	switch eventType {
	case EV_SYN:
		return "EV_SYN"
	case EV_KEY:
		return "EV_KEY"
	case EV_REL:
		return "EV_REL"
	case EV_ABS:
		return "EV_ABS"
	case EV_MSC:
		return "EV_MSC"
	case EV_SW:
		return "EV_SW"
	case EV_LED:
		return "EV_LED"
	case EV_SND:
		return "EV_SND"
	case EV_REP:
		return "EV_REP"
	case EV_FF:
		return "EV_FF"
	case EV_PWR:
		return "EV_PWR"
	case EV_FF_STATUS:
		return "EV_FF_STATUS"
	default:
		return fmt.Sprintf("UNKNOWN_%d", eventType)
	}
}

func GetEventTypeName(eventTypeName string) (EventType, bool) {
	switch eventTypeName {
	case "EV_SYN":
		return EV_SYN, true
	case "EV_KEY":
		return EV_KEY, true
	case "EV_REL":
		return EV_REL, true
	case "EV_ABS":
		return EV_ABS, true
	case "EV_MSC":
		return EV_MSC, true
	case "EV_SW":
		return EV_SW, true
	case "EV_LED":
		return EV_LED, true
	case "EV_SND":
		return EV_SND, true
	case "EV_REP":
		return EV_REP, true
	case "EV_FF":
		return EV_FF, true
	case "EV_PWR":
		return EV_PWR, true
	case "EV_FF_STATUS":
		return EV_FF_STATUS, true
	default:
		return 0, false
	}
}

func GetKeyName(code uint16) string {
	if name, exists := keyCodes[code]; exists {
		return name
	}
	return fmt.Sprintf("KEY_%d", code)
}

func GetRelName(code uint16) string {
	if name, exists := RelCodes[code]; exists {
		return name
	}
	return fmt.Sprintf("REL_%d", code)
}

func GetAbsName(code uint16) string {
	if name, exists := AbsCodes[code]; exists {
		return name
	}
	return fmt.Sprintf("ABS_%d", code)
}

func GetKeyStateName(value int32) string {
	switch value {
	case KEY_RELEASED:
		return "RELEASED"
	case KEY_PRESSED:
		return "PRESSED"
	case KEY_REPEATED:
		return "REPEATED"
	default:
		return fmt.Sprintf("UNKNOWN_%d", value)
	}
}
