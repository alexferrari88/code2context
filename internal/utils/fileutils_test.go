package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFileSize_ValidInputs(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int64
	}{
		{"bytes only", "1024", 1024},
		{"KB", "2KB", 2 * KB},
		{"MB", "3MB", 3 * MB},
		{"GB", "4GB", 4 * GB},
		{"TB", "1TB", 1 * TB},
		{"kilobytes with B", "2KB", 2 * KB},
		{"megabytes with B", "3MB", 3 * MB},
		{"gigabytes with B", "4GB", 4 * GB},
		{"terabytes with B", "1TB", 1 * TB},
		{"lowercase kb", "2kb", 2 * KB},
		{"lowercase mb", "3mb", 3 * MB},
		{"lowercase gb", "4gb", 4 * GB},
		{"lowercase tb", "1tb", 1 * TB},
		{"with space", "5 MB", 5 * MB},
		{"float KB", "1.5KB", int64(1.5 * float64(KB))},
		{"float MB", "2.5MB", int64(2.5 * float64(MB))},
		{"float GB", "0.5GB", int64(0.5 * float64(GB))},
		{"float TB", "0.25TB", int64(0.25 * float64(TB))},
		{"zero value", "0", 0},
		{"zero value with unit", "0KB", 0},
		// Test near max int64, but not overflowing
		{"near max int64 bytes", "9223372036854775806", 9223372036854775806},
		{"near max int64 KB", "9007199254740KB", 9007199254740 * KB}, // approx 8191TB, (2^53-1)*2^10
		// MaxInt64 is 9223372036854775807. MaxInt64 / TB (2^40) = 8388607.99...
		// So 8388607TB is the largest whole number of TBs that fits in int64.
		{"max int64 as TB string", "8388607TB", 8388607 * TB},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size, err := ParseFileSize(tc.input)
			require.NoError(t, err, "ParseFileSize returned an unexpected error for valid input: %s", tc.input)
			assert.Equal(t, tc.expected, size, "Parsed size does not match expected value for input: %s", tc.input)
		})
	}
}

func TestParseFileSize_InvalidInputs(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedError string // Substring of the expected error message
	}{
		{"empty string", "", "file size string is empty"},
		{"invalid format", "abc", "invalid file size format"},
		{"invalid numeric value for unit regex", "abcKB", "invalid file size format"},
		{"unknown unit for unit regex", "5XB", "invalid file size format"},
		{"just unit", "MB", "invalid file size format"},
		{"negative value", "-1MB", "invalid file size format"},
		{"invalid float for unit regex", "1.2.3MB", "invalid file size format"},
		{"no numeric part", "KB", "invalid file size format"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			size, err := ParseFileSize(tc.input)
			require.Error(t, err, "ParseFileSize should have returned an error for input: %s", tc.input)
			if err != nil { // Check err is not nil before using it
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.expectedError), "Error message mismatch for input: %s", tc.input)
			}
			assert.Equal(t, int64(0), size, "Size should be 0 on error for input: %s", tc.input)
		})
	}
}

func TestParseFileSize_Overflow(t *testing.T) {
	// 8388608 TB = 2^23 * 2^40 bytes = 2^63 bytes. This is math.MaxInt64 + 1.
	// float64(8388608 * TB) will be exactly 9.223372036854776e+18
	// math.MaxInt64 is 9223372036854775807
	// So, 8388608TB should cause an overflow.
	input := "8388608TB" // This should cause overflow
	expectedErrorMsg := "overflows int64"

	size, err := ParseFileSize(input)

	require.Errorf(t, err, "ParseFileSize should return an error for overflow with input: %s. Got size: %d, err: %v", input, size, err)
	if err != nil {
		assert.Contains(t, err.Error(), expectedErrorMsg, "Error message does not contain expected overflow string for input: %s", input)
	}
	assert.Equal(t, int64(0), size, "Size should be 0 on overflow error for input: %s", input)

	// Test with MaxInt64 bytes itself, should pass (this was in ValidInputs, moved here for direct comparison)
	maxInt64Str := "9223372036854775807"
	sizeMax, errMax := ParseFileSize(maxInt64Str)
	require.NoError(t, errMax, "Parsing MaxInt64 as bytes string should not error")
	assert.Equal(t, int64(9223372036854775807), sizeMax, "Parsed MaxInt64 bytes string incorrect")

	// Test one byte more than MaxInt64, should fail via plain bytes path
	// However, the regex for plain bytes might not allow such a large number to be parsed by strconv.ParseInt if it's too long.
	// strconv.ParseInt("9223372036854775808", 10, 64) itself would error.
	// Let's test this specific path.
	// The current regex `^(\d+)$` would capture it, then `strconv.ParseInt` would fail.
	// The error from `strconv.ParseInt` for "value out of range" is what we'd expect.
	oneOverMaxStr := "9223372036854775808"
	sizeOneOver, errOneOver := ParseFileSize(oneOverMaxStr)
	require.Error(t, errOneOver, "Parsing one byte over MaxInt64 should error")
	// The error comes from strconv.ParseInt: "value out of range", wrapped by "invalid plain byte size"
	assert.Contains(t, errOneOver.Error(), "value out of range", "Error for one over max bytes not as expected")
	assert.Contains(t, errOneOver.Error(), "invalid plain byte size", "Error for one over max bytes not as expected")
	assert.Equal(t, int64(0), sizeOneOver, "Size should be 0 on one over max bytes error")
}
