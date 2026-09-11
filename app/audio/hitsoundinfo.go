package audio

type HitSoundInfo struct {
	SampleSet    int
	AdditionSet  int
	CustomIndex  int
	CustomVolume float64
	Filename     string
}

type HitSound struct {
	Sample int
	Info   HitSoundInfo
}
