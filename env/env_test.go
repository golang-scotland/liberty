package env

import (
	"testing"
	//	. "github.com/smartystreets/goconvey/convey"
)

func TestSetEnv(t *testing.T) {

	/*
		Convey("Default env is dev", t, func() {
			So(Get(), ShouldEqual, Dev)
		})

		Convey("Given a valid env value", t, func() {
			e := Env(Prod)

			Convey("Setting the current env is successfull", func() {
				So(Set(e), ShouldEqual, nil)
				So(Get(), ShouldHaveSameTypeAs, e)
				So(Get(), ShouldEqual, Prod)
			})
		})

		Convey("Given an invalid env value", t, func() {
			failEnv := Env(666)

			Convey("Setting the current env returns an error", func() {
				So(Set(failEnv), ShouldNotEqual, nil)
				So(Set(failEnv), ShouldHaveSameTypeAs, ErrInvalidValue)
			})
		})
	*/
}

func TestTheEnvironmentIsSafeToRunIn(t *testing.T) {
	/*

		Convey("There should not be any global proxy set", t, func() {
			Printf("http_proxy: %s", os.Getenv("http_proxy"))
			So(os.Getenv("http_proxy"), ShouldEqual, "")
			So(os.Getenv("PROXY"), ShouldEqual, "")
		})

		os.Setenv("http_proxy", "foo")
		os.Setenv("PROXY", "foo")

		Convey("Safe returns false if proxy servers are set", t, func() {
			So(Safe(), ShouldBeFalse)

			Convey("Unset global proxy variable if it exists", func() {
				MakeSafe()
				So(os.Getenv("http_proxy"), ShouldEqual, "")
				So(os.Getenv("PROXY"), ShouldEqual, "")
				So(Safe(), ShouldBeTrue)
			})
		})
	*/

}
