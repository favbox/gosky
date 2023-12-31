package stringx

import (
	"github.com/favbox/gosky/air/gopkg/lang/fastrand"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Randn 返回长度为 n 的随机字符串。
func Randn(n int) []int {
	return fastrand.Perm(n)
}
