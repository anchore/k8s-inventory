package util

import "testing"

var obfuscateCases = []struct {
	input    string
	expected string
}{
	{
		input:    "password: xyz",
		expected: "password: ******",
	},
	{
		input:    "token: def",
		expected: "token: ******",
	},
	{
		input:    "key: abc",
		expected: "key: ******",
	},
	{
		input:    "private_key: abc",
		expected: "private_key: ******",
	},
	{
		input:    "password: abc\nprivate_key: xyz\nsomething: \"something_else\"\rtoken: vvv",
		expected: "password: ******\nprivate_key: ******\nsomething: \"something_else\"\rtoken: ******",
	},
}

func TestObfuscateSensitiveString(t *testing.T) {
	for _, testCase := range obfuscateCases {
		actual := ObfuscateSensitiveString(testCase.input)
		if actual != testCase.expected {
			t.Errorf("obfuscation failed:\nexpected: %s\n\nactual: %s", testCase.expected, actual)
		}
	}
}
