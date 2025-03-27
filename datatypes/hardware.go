package datatypes

type ButtonType int
const (
	BT_HallUP ButtonType = iota
	BT_HallDOWN
	BT_CAB
)

type ButtonEvent struct {
	Floor  int
	Button ButtonType
}

type MotorDirection int
const (
	MD_UP   MotorDirection = 1
	MD_DOWN MotorDirection = -1
	MD_STOP MotorDirection = 0
)
