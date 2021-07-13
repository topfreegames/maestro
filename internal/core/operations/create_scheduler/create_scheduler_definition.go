package create_scheduler

type CreateSchedulerDefinition struct {}

func (d *CreateSchedulerDefinition) Name() string {
	return "create_scheduler"
}

func (d *CreateSchedulerDefinition) Marshal() []byte {		
	return make([]byte, 0)
}

func (d *CreateSchedulerDefinition) Unmarshal(raw []byte) error {
	return nil
}
