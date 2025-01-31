package utils

import (
	"fmt"
	"github.com/shopspring/decimal"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func StandardizeString(str_ string) (string, error) {
	return ConvertStringTo18DecimalPlaces(str_)
}

func StandardizeFloatToString(fl_ float64) (string, error) {
	str_ := FloatToString(fl_)
	return ConvertStringTo18DecimalPlaces(str_)
}

func SanitizeStringLetters(str string) string {
	newS := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, str)

	return newS
}

func PruneStringDecimals(str string, dec_ int) string {
	idx := strings.Index(str, ".")
	if idx < 0 {
		str = str + ".0000"
	}
	decimalPart := str[idx+1:]

	if len(decimalPart) > dec_ {
		decimalPart = decimalPart[:dec_]
	} else {
		needsZeroes := dec_ - len(decimalPart)
		for i := 0; i < needsZeroes; i++ {
			decimalPart += "0"
		}
	}
	return str[:idx+1] + decimalPart
}

func PruneStringByDecimalPoints(value string, decimalPoints int) (string, error) {
	// Parse the string value as a float64
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "", err
	}

	// Format the float64 value with the desired decimal points
	prunedValue := strconv.FormatFloat(number, 'f', decimalPoints, 64)

	return prunedValue, nil
}

func ConvertStringTo18DecimalPlaces(str string) (string, error) {
	idx := strings.Index(str, ".")
	if idx < 0 {
		str = str + ".0000"
		idx = strings.Index(str, ".")
	}

	decimalPart := str[idx+1:]
	needsZeroes := 18 - len(decimalPart)
	for i := 0; i < needsZeroes; i++ {
		decimalPart += "0"
	}
	return str[:idx+1] + decimalPart, nil
}

func ConvertFloatTo18DecimalPlaces(f float64) (string, error) {
	str := fmt.Sprintf("%.4f", f)
	idx := strings.Index(str, ".")
	if idx < 0 {
		str = str + ".0000"
	}
	decimalPart := str[idx+1:]
	needsZeroes := 18 - len(decimalPart)
	for i := 0; i < needsZeroes; i++ {
		decimalPart += "0"
	}
	return str[:idx+1] + decimalPart, nil
}

func FloatToString(f float64) string {
	d, _ := ConvertStringTo18DecimalPlaces(decimal.NewFromFloat(f).String())
	return d
}

// TODO: Add more intelligent fuzzing so that if there is a small chance of an overlapping dono (exact same fuzzing and dono) then the dono is fuzzed again
// this will almost never happen but it's better than leaving it up to chance
func FuzzDono(ethAmount float64, cryptoCode string) float64 {
	if cryptoCode == "HEX" || cryptoCode == "SOL" {
		// generate random value between 0 and 1 millionth
		rand.Seed(time.Now().UnixNano())
		randVal := rand.Float64() / 1000.0

		// add random value to input amount
		newAmount := ethAmount + randVal

		return newAmount
	} else {
		// generate random value between 0 and 1 millionth
		rand.Seed(time.Now().UnixNano())
		randVal := rand.Float64() / 10000000.0

		// add random value to input amount
		newAmount := ethAmount + randVal

		return newAmount
	}
}
