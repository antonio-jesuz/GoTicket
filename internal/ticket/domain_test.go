package ticket

import "testing"

func TestTransitions(t *testing.T) {
	cases := []struct {
		from, to Status
		ok       bool
	}{
		{StatusNew, StatusOpen, true},
		{StatusNew, StatusResolved, false},
		{StatusOpen, StatusInProgress, true},
		{StatusResolved, StatusOpen, true}, // reabertura
		{StatusResolved, StatusClosed, true},
		{StatusClosed, StatusOpen, false}, // fechado é terminal
	}
	for _, c := range cases {
		if got := c.from.CanMoveTo(c.to); got != c.ok {
			t.Errorf("%s → %s: esperado %v, obtido %v", c.from, c.to, c.ok, got)
		}
	}
}

func TestPrioritySLA(t *testing.T) {
	if !PriorityCritical.Valid() || PriorityCritical.SLA() >= PriorityLow.SLA() {
		t.Error("SLA crítico deve ser menor que o de baixa prioridade")
	}
	if Priority("x").Valid() {
		t.Error("prioridade desconhecida não deve ser válida")
	}
}
