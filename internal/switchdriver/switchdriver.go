package switchdriver

type (
	Switch interface {
		TurnOn() error
		TurnOff() error
		String() string
	}

	SwitchCollection interface {
		CountSwitches() uint
		ListSwitches() []Switch
		GetSwitch(id uint) (Switch, error)
		TurnOn() error
		TurnOff() error
		Init() error
		Close() error
		String() string
	}
)
