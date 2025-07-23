package switchcollection

type (
	Switch interface {
		TurnOn() error
		TurnOff() error
		GetState() (bool, error)
		IsDisabled() bool
		String() string
	}

	SwitchCollection interface {
		Switch
		CountSwitches() uint
		ListSwitches() []Switch
		GetSwitch(id uint) (Switch, error)
		GetDetailedState() ([]bool, error)
		Init() error
		Close() error
	}
)
