package memdb

import "time"

type ageExpirer struct {
	cTime time.Duration
	aTime time.Duration
	mTime time.Duration
}

func AgeExpirer(cTime, mTime, aTime time.Duration) Expirer {
	return &ageExpirer{
		cTime: cTime,
		aTime: aTime,
		mTime: mTime,
	}
}

func (ae *ageExpirer) IsExpired(a interface{}, now time.Time, stats Stats) bool {
	if ae.cTime != 0 && now.Sub(stats.Created) > ae.cTime {
		return true
	}
	if ae.aTime != 0 && now.Sub(stats.Accessed) > ae.aTime {
		return true
	}
	if ae.mTime != 0 && now.Sub(stats.Modified) > ae.mTime {
		return true
	}
	return false
}
