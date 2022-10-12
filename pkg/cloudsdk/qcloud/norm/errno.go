package norm

import (
	"fmt"
)

type Errno struct {
	Code int
	Msg  string
}

func (e Errno) GenMsg(msg string) string {
	return e.Msg + "[" + msg + "]"
}

//DB相关错误

// http client相关错误
var COMPONENT_CLINET_COMMON_ERROR = Errno{Code: -30000, Msg: "COMPONENT_CLINET_COMMON_ERROR"}
var COMPONENT_CLINET_HTTP_ERROR = Errno{Code: -30001, Msg: "COMPONENT_CLINET_HTTP_ERROR"}
var COMPONENT_CLINET_UNPACK_ERROR = Errno{Code: -30002, Msg: "COMPONENT_CLINET_UNPACK_ERROR"}

// TODO DashboardError中增加 嵌套的DashboardError类型,用来一层层传递错误
// 如 requestResultError -> norm Error ->master create Error
type DashboardError struct {
	Code int
	Msg  string
	Err  error
}

func (e DashboardError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("DashboardError,Code : %d , Msg : %s, err : %s", e.Code, e.Msg, e.Err.Error())
	}
	return fmt.Sprintf("DashboardError,Code : %d , Msg : %s, err : nil", e.Code, e.Msg)
}

func NewDashboardError(errno Errno, extraMsg string, err error) *DashboardError {
	return &DashboardError{Code: errno.Code, Msg: errno.GenMsg(extraMsg), Err: err}
}
