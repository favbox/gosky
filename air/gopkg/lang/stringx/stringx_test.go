package stringx

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestPad(t *testing.T) {
	type testData struct {
		input             string
		padChar           rune
		size              int
		leftExpected      string
		leftExpectedSpace string

		rightExpected      string
		rightExpectedSpace string

		centerExpected      string
		centerExpectedSpace string
	}

	testCases := []testData{
		{
			"", '-', 4,
			"----", "    ",
			"----", "    ",
			"----", "    ",
		},
		{
			"abc", '-', 0,
			"abc", "abc",
			"abc", "abc",
			"abc", "abc",
		},
		{
			"abc", '-', 2,
			"abc", "abc",
			"abc", "abc",
			"abc", "abc",
		},
		{
			"abc", '-', 4,
			"-abc", " abc",
			"abc-", "abc ",
			"abc-", "abc ",
		},
		{
			"abc", '-', 5,
			"--abc", "  abc",
			"abc--", "abc  ",
			"-abc-", " abc ",
		},
		{
			"abc", '-', 6,
			"---abc", "   abc",
			"abc---", "abc   ",
			"-abc--", " abc  ",
		},
		{
			"abc", '-', 7,
			"----abc", "    abc",
			"abc----", "abc    ",
			"--abc--", "  abc  ",
		},

		{
			"abcd", '-', 7,
			"---abcd", "   abcd",
			"abcd---", "abcd   ",
			"-abcd--", " abcd  ",
		},
	}

	is := assert.New(t)
	for _, testCase := range testCases {
		is.Equal(testCase.leftExpected, PadLeftRune(testCase.input, testCase.size, testCase.padChar))
		is.Equal(testCase.leftExpectedSpace, PadLeftSpace(testCase.input, testCase.size))

		is.Equal(testCase.rightExpected, PadRightRune(testCase.input, testCase.size, testCase.padChar))
		is.Equal(testCase.rightExpectedSpace, PadRightSpace(testCase.input, testCase.size))

		is.Equal(testCase.centerExpected, PadCenterRune(testCase.input, testCase.size, testCase.padChar))
		is.Equal(testCase.centerExpectedSpace, PadCenterSpace(testCase.input, testCase.size))
	}
}

func TestRemove(t *testing.T) {
	is := assert.New(t)
	is.Equal("", RemoveRune("", 'h'))
	is.Equal("z英文un排", RemoveRune("zh英文hunh排", 'h'))
	is.Equal("zh英hun排", RemoveRune("zh英文hun文排", '文'))

	is.Equal("", RemoveString("", "文hun"))
	is.Equal("zh英文hun排", RemoveString("zh英文hun排", ""))
	is.Equal("zh英排", RemoveString("zh英文hun排", "文hun"))
	is.Equal("zh英文hun排", RemoveString("zh英文hun排", ""))
}

func TestRepeat(t *testing.T) {
	is := assert.New(t)
	is.Equal("", RepeatRune('-', 0))
	is.Equal("----", RepeatRune('-', 4))
	is.Equal("   ", RepeatRune(' ', 3))
}

func TestRotate(t *testing.T) {
	is := assert.New(t)

	is.Equal("", Rotate("", 2))

	is.Equal("abc", Rotate("abc", 0))
	is.Equal("abc", Rotate("abc", 3))
	is.Equal("abc", Rotate("abc", 6))

	is.Equal("cab", Rotate("abc", 1))
	is.Equal("bca", Rotate("abc", -1))
	is.Equal("你我爱", Rotate("我爱你", 1))
	is.Equal("爱你我", Rotate("我爱你", -1))
}

func TestReverse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "cba"},
		{"a", "a"},
		{"çınar", "ranıç"},
		{"    yağmur", "rumğay    "},
		{"επαγγελματίες", "ςείταμλεγγαπε"},
	}
	for _, test := range tests {
		output := MustReverse(test.input)
		assert.Equalf(t, test.expected, output, "Test case %s is not successful\n", test.input)
	}

	assert.Equal(t, MustReverse(""), "")
	assert.Equal(t, MustReverse("X"), "X")
	assert.Equal(t, MustReverse("\u0301b"), "b\u0301")
	assert.Equal(t, MustReverse("😎⚽"), "⚽😎")
	assert.Equal(t, MustReverse("Les Mise\u0301rables"), "selbar\u0301esiM seL")
	assert.Equal(t, MustReverse("ab\u0301cde"), "edc\u0301ba")
	assert.Equal(t, MustReverse("The quick bròwn 狐 jumped over the lazy 犬"), "犬 yzal eht revo depmuj 狐 nwòrb kciuq ehT")
	s := string([]byte{128, 128, 128, 128, 0})
	//fmt.Println("解码失败字符串", s)
	_, err := Reverse(s)
	assert.Equal(t, ErrDecodeRune, err)
}

func TestSub(t *testing.T) {
	type testData struct {
		input    string
		start    int
		end      int
		expected string
	}

	newTestCase := func(input string, start, end int, expected string) testData {
		return testData{
			input:    input,
			start:    start,
			end:      end,
			expected: expected,
		}
	}

	testCases := []testData{
		newTestCase("", 0, 100, ""),
		newTestCase("facgbheidjk", 3, 9, "gbheid"),
		newTestCase("facgbheidjk", -50, 100, "facgbheidjk"),
		newTestCase("facgbheidjk", -3, utf8.RuneCountInString("facgbheidjk"), "djk"),
		newTestCase("facgbheidjk", -3, -1, "dj"),
		newTestCase("zh英文hun排", 2, 5, "英文h"),
		newTestCase("zh英文hun排", 2, -1, "英文hun"),
		newTestCase("zh英文hun排", -100, -1, "zh英文hun"),
		newTestCase("zh英文hun排", -100, -90, ""),
		newTestCase("zh英文hun排", -10, -90, ""),
	}

	is := assert.New(t)
	for _, testCase := range testCases {
		is.Equal(testCase.expected, Sub(testCase.input, testCase.start, testCase.end))
	}
}

func TestContainsAnySubstrings(t *testing.T) {
	is := assert.New(t)
	is.True(ContainsAnySubstrings("abcdefg", []string{"a", "b"}))
	is.True(ContainsAnySubstrings("abcdefg", []string{"a", "z"}))
	is.False(ContainsAnySubstrings("abcdefg", []string{"ac", "z"}))
	is.False(ContainsAnySubstrings("abcdefg", []string{"x", "z"}))
}

func TestShuffle(t *testing.T) {
	is := assert.New(t)

	shuffleAndSort := func(str string) string {
		s := Shuffle(str)
		slice := sort.StringSlice(strings.Split(s, ""))
		slice.Sort()
		return strings.Join(slice, "")
	}

	strMap := map[string]string{
		"":            "",
		"facgbheidjk": "abcdefghijk",
		"尝试中文":        "中尝文试",
		"zh英文hun排":    "hhnuz排文英",
	}
	for input, expected := range strMap {
		actual := shuffleAndSort(input)
		is.Equal(expected, actual)
	}
}

func TestPadd(t *testing.T) {
	fmt.Printf("[%s]\n", PadLeftRune("中国", 5, '-'))
	fmt.Printf("[%s]\n", PadLeftRune("abc", 5, '-'))
}
