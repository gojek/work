package work

import "time"

var nowMock int64

func nowEpochSeconds() int64 {
	if nowMock != 0 {
		return nowMock
	}
	return time.Now().Unix()
}

func setNowEpochSecondsMock(t int64) {
	nowMock = t
}

func resetNowEpochSecondsMock() {
	nowMock = 0
}

func epochAfterSeconds(seconds int64) int64 {
	if nowMock != 0 {
		return nowMock + seconds
	}
	t := time.Now().Add(time.Second * time.Duration(seconds))
	if t.Nanosecond() > 0 {
		t = t.Add(time.Second)
	}
	return t.Unix()
}
