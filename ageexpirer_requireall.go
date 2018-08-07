package memdb

import "time"

type ageExpirerRequireAll struct {
	cTime time.Duration
	aTime time.Duration
	mTime time.Duration
	cb    []ExpireFunc
}

// AgeExpirerRequireAll is an Expirer that checks the provided times since create/last modify/last access and
// the provided ExpireFunc's and marks the item as expired only if all provided values are true
func AgeExpirerRequireAll(cTime, mTime, aTime time.Duration, cb ...ExpireFunc) Expirer {
	return &ageExpirerRequireAll{
		cTime: cTime,
		aTime: aTime,
		mTime: mTime,
		cb:    cb,
	}
}

// IsExpired implements the necessary function for an Expirer
func (ae *ageExpirerRequireAll) IsExpired(a interface{}, now time.Time, stats Stats) bool {
	cTime := stats.Created
	mTime := stats.Modified
	if mTime.IsZero() {
		mTime = cTime
	}
	aTime := stats.Accessed
	if aTime.IsZero() {
		aTime = mTime
	}

	expired := true
	if ae.cTime != 0 && now.Sub(cTime) < ae.cTime {
		expired = false
	}
	if ae.aTime != 0 && now.Sub(aTime) < ae.aTime {
		expired = false
	}
	if ae.mTime != 0 && now.Sub(mTime) < ae.mTime {
		expired = false
	}

	for _, cb := range ae.cb {
		if v := cb(a, now, stats); v == ExpireFalse {
			expired = false
		} else if v == ExpireTrue {
			expired = true
		}
	}
	return expired
}
