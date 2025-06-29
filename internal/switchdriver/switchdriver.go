package switchdriver

type (
	Switch interface {
		TurnOn() error
		TurnOff() error
		GetState() (bool, error)
		String() string
	}

	SwitchCollection interface {
		CountSwitches() uint
		ListSwitches() []Switch
		GetSwitch(id uint) (Switch, error)
		Close() error
		String() string
	}
)
