package datatypes

type MotorDirection int

const (
	MD_UP   MotorDirection = 1
	MD_DOWN MotorDirection = -1
	MD_STOP MotorDirection = 0
)

type ButtonType int

const (
	BT_HallUP   ButtonType = 1
	BT_HallDOWN ButtonType = 0
	BT_CAB      ButtonType = 2
)

type ButtonEvent struct {
	Floor  int
	Button ButtonType
}
