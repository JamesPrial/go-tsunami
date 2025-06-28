package common

type ParseInstructionError struct {
	Instruction string
}

func (e *ParseInstructionError) Error() string {
	return e.Instruction
}
