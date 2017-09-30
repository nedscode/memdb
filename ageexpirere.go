package memdb

import "time"

type ageExpirer struct {
	cTime time.Duration
	aTime time.Duration
	mTime time.Duration
}

// AgeExpirer is an Expirer that works purely by time since create/last modify/last access
func AgeExpirer(cTime, mTime, aTime time.Duration) Expirer {
	return &ageExpirer{
		cTime: cTime,
		aTime: aTime,
		mTime: mTime,
	}
}

// IsExpired implements the necessary function for an Expirer
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
