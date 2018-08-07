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

	cTime := stats.Created
	mTime := stats.Modified
	if mTime.IsZero() {
		mTime = cTime
	}
	aTime := stats.Accessed
	if aTime.IsZero() {
		aTime = mTime
	}

	if ae.cTime != 0 && now.Sub(cTime) > ae.cTime {
		return true
	}
	if ae.aTime != 0 && now.Sub(aTime) > ae.aTime {
		return true
	}
	if ae.mTime != 0 && now.Sub(mTime) > ae.mTime {
		return true
	}
	return false
}
