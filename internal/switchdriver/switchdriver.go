package switchdriver

type (
	Switch interface {
		TurnOn() error
		TurnOff() error
		GetState() (bool, error)
		String() string
	}

	SwitchCollection interface {
		ListSwitches() ([]Switch, error)
		GetSwitch(id uint) (Switch, error)
		Close() error
		String() string
	}
)
