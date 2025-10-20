package combat

type Event struct {
	T       float64        `json:"t"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Status struct {
	Name     string
	ExpireAt float64
}

type Entity struct {
	ID       string
	HP       int
	MaxHP    int
	Guard    int
	GuardMax int

	Pos     Vec2 // 位置
	Speed   float64
	Range   float64
	AtkCD   float64
	nextAtk float64

	Resist   map[string]float64
	Weakness map[string]bool
	Statuses map[string]Status
	Tags     map[string]bool

	BuffDMGTakenMul float64
}

func NewBoss(id string, maxHP, guardMax int) *Entity {
	return &Entity{
		ID: id, HP: maxHP, MaxHP: maxHP, Guard: guardMax, GuardMax: guardMax,
		Resist: map[string]float64{}, Weakness: map[string]bool{},
		Statuses: map[string]Status{}, Tags: map[string]bool{},
		BuffDMGTakenMul: 1.0,
		Pos:             Vec2{X: 10, Y: 5},
		Speed:           2.0, Range: 3.0, AtkCD: 1.2,
	}
}
