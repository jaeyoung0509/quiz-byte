package models

import (
	"database/sql/driver"
	"reflect"
	"testing"
)

// stringDelimiter is defined in quiz.go

func TestStringSlice_Value(t *testing.T) {
	tests := []struct {
		name    string
		s       StringSlice
		wantVal driver.Value
		wantErr bool
	}{
		{
			name:    "nil slice",
			s:       nil,
			wantVal: "",
			wantErr: false,
		},
		{
			name:    "empty slice",
			s:       StringSlice{},
			wantVal: "",
			wantErr: false,
		},
		{
			name:    "slice with one element",
			s:       StringSlice{"apple"},
			wantVal: "apple",
			wantErr: false,
		},
		{
			name:    "slice with multiple elements",
			s:       StringSlice{"apple", "banana"},
			wantVal: "apple" + stringDelimiter + "banana",
			wantErr: false,
		},
		{
			name:    "slice with element containing delimiter",
			s:       StringSlice{"part1|||part2", "orange"},
			wantVal: "part1|||part2" + stringDelimiter + "orange",
			wantErr: false,
		},
		{
			name:    "slice with empty string element",
			s:       StringSlice{"", "test"},
			wantVal: "" + stringDelimiter + "test",
			wantErr: false,
		},
		{
			name:    "slice with multiple empty string elements",
			s:       StringSlice{"", ""},
			wantVal: "" + stringDelimiter + "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, err := tt.s.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("StringSlice.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotVal, tt.wantVal) {
				t.Errorf("StringSlice.Value() gotVal = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}

func TestStringSlice_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantS   StringSlice
		wantErr bool
	}{
		{
			name:    "nil input",
			value:   nil,
			wantS:   StringSlice{},
			wantErr: false,
		},
		{
			name:    "empty string input",
			value:   "",
			wantS:   StringSlice{},
			wantErr: false,
		},
		{
			name:    "single element string",
			value:   "apple",
			wantS:   StringSlice{"apple"},
			wantErr: false,
		},
		{
			name:    "multiple elements string",
			value:   "apple" + stringDelimiter + "banana",
			wantS:   StringSlice{"apple", "banana"},
			wantErr: false,
		},
		{
			name:    "string with delimiter as part of element", // "part1|||part2|||orange"
			value:   "part1" + stringDelimiter + "part2" + stringDelimiter + "orange", // This is "part1|||part2|||orange"
			wantS:   StringSlice{"part1", "part2", "orange"}, // Because strings.Split("part1|||part2|||orange", "|||") is {"part1", "part2", "orange"}
			wantErr: false,
		},
		{
			name:    "leading delimiter", // "|||test" -> {"", "test"}
			value:   stringDelimiter + "test",
			wantS:   StringSlice{"", "test"},
			wantErr: false,
		},
		{
			name:    "trailing delimiter", // "test|||" -> {"test", ""}
			value:   "test" + stringDelimiter,
			wantS:   StringSlice{"test", ""},
			wantErr: false,
		},
		{
			name:    "double delimiter (empty string element in between)", // "apple||||||banana" -> {"apple", "", "banana"}
			value:   "apple" + stringDelimiter + stringDelimiter + "banana",
			wantS:   StringSlice{"apple", "", "banana"},
			wantErr: false,
		},
		{
			name:    "empty byte slice input",
			value:   []byte(""),
			wantS:   StringSlice{},
			wantErr: false,
		},
		{
			name:    "byte slice with multiple elements",
			value:   []byte("apple" + stringDelimiter + "banana"),
			wantS:   StringSlice{"apple", "banana"},
			wantErr: false,
		},
		{
			name:    "unsupported type int",
			value:   int(123),
			wantS:   nil, // or StringSlice{} depending on how error cases are handled by Scan
			wantErr: true,
		},
		{
            name:    "single empty string from db (results in one empty string in slice after split)",
            value:   "", // This is already covered by "empty string input" which expects StringSlice{}
            wantS:   StringSlice{},
            wantErr: false,
        },
		{
            name:    "string that splits into a single empty string", // e.g. if db somehow stores just "|||" but means one empty element
            value:   stringDelimiter, // this would split into {"", ""}
            wantS:   StringSlice{"", ""}, // current Scan logic: strings.Split("|||", "|||") -> ["", ""]
            wantErr: false,
        },

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s StringSlice
			err := s.Scan(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringSlice.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Only compare s if no error is expected, or if wantS is explicitly set for error cases (though typically it's nil/zero-value)
			if !tt.wantErr && !reflect.DeepEqual(s, tt.wantS) {
				t.Errorf("StringSlice.Scan() gotS = %v, want %v", s, tt.wantS)
			}
			// If an error is expected and wantS is nil (or zero value), this check is fine.
			// If wantS was something specific for an error case, adjust logic.
			if tt.wantErr && tt.wantS != nil && !reflect.DeepEqual(s, tt.wantS) {
                 // This case might be relevant if Scan modified s before returning an error,
                 // but typically s should be in a predictable (e.g. zero) state.
                 t.Errorf("StringSlice.Scan() gotS = %v with error, want %v", s, tt.wantS)
            }
		})
	}
}
