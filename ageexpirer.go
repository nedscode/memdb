package memdb

import "time"

type ageExpirer struct {
	cTime time.Duration
	aTime time.Duration
	mTime time.Duration
	cb    []ExpireFunc
}

// AgeExpirer is an Expirer that works by time since create/last modify/last access with an optional array of ExpireFunc's
// which if provided will be checked first
func AgeExpirer(cTime, mTime, aTime time.Duration, cb ...ExpireFunc) Expirer {
	return &ageExpirer{
		cTime: cTime,
		aTime: aTime,
		mTime: mTime,
		cb:    cb,
	}
}

// IsExpired implements the necessary function for an Expirer
func (ae *ageExpirer) IsExpired(a interface{}, now time.Time, stats Stats) bool {
	for _, cb := range ae.cb {
		if v := cb(a, now, stats); v != ExpireNull {
			return v == ExpireTrue
		}
	}
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
