package switchdriver

type (
	Switch interface {
		TurnOn() error
		TurnOff() error
		GetState() (bool, error)
		GetID() uint
		String() string
	}

	SwitchCollection interface {
		CountSwitches() uint
		ListSwitches() []Switch
		GetSwitch(id uint) (Switch, error)
		TurnAllOff() error
		TurnAllOn() error
		Close() error
		String() string
	}
)
