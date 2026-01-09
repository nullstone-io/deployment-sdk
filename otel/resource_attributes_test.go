package otel

import (
	"testing"
)

func TestParseResourceAttributes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ResourceAttributes
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "whitespace only",
			input: "   \t  ",
			want:  map[string]string{},
		},
		{
			name:  "single pair",
			input: "service.name=api",
			want:  map[string]string{"service.name": "api"},
		},
		{
			name:  "trims whitespace around key/value",
			input: " service.name = api , service.version = 1 ",
			want:  map[string]string{"service.name": "api", "service.version": "1"},
		},
		{
			name:  "ignores empty entries between commas",
			input: "service.name=api,,service.version=1,",
			want:  map[string]string{"service.name": "api", "service.version": "1"},
		},
		{
			name:  "duplicate keys last wins",
			input: "service.name=api,service.name=api2",
			want:  map[string]string{"service.name": "api2"},
		},
		{
			name:  "escaped comma in value",
			input: `team=platform\,core`,
			want:  map[string]string{"team": "platform,core"},
		},
		{
			name:  "escaped equals in value",
			input: `token=a\=b`,
			want:  map[string]string{"token": "a=b"},
		},
		{
			name:  "escaped backslash in value",
			input: `path=C:\\apps\\api`,
			want:  map[string]string{"path": `C:\apps\api`},
		},
		{
			name:  "escaped delimiters in key",
			input: `weird\,key=val,weird\=key2=val2,weird\\key3=val3`,
			want: map[string]string{
				"weird,key":  "val",
				"weird=key2": "val2",
				`weird\key3`: "val3",
			},
		},
		{
			name:  "split on first unescaped equals",
			input: `a=b=c`,
			want:  map[string]string{"a": "b=c"},
		},
		{
			name:  "split does not break on escaped equals",
			input: `a=b\=c=d`,
			want:  map[string]string{"a": "b=c=d"},
		},
		{
			name:  "split does not break on escaped comma",
			input: `a=b\,c,d=e`,
			want:  map[string]string{"a": "b,c", "d": "e"},
		},
		{
			name:  "trailing backslash in value is ignored (per implementation)",
			input: `a=b\`,
			want:  map[string]string{"a": "b"},
		},
		{
			name:  "missing equals is an error",
			input: "service.name",
			// error because no '=' present in pair
			wantErr: true,
		},
		{
			name:  "empty key is an error",
			input: "=value",
			// error because key becomes empty
			wantErr: true,
		},
		{
			name:  "key only whitespace is an error",
			input: "   =value",
			// trims -> empty key
			wantErr: true,
		},
		{
			name:  "value may be empty",
			input: "a=",
			want:  map[string]string{"a": ""},
		},
		{
			name:  "pair with only spaces around comma is ignored",
			input: "a=b,   ,c=d",
			want:  map[string]string{"a": "b", "c": "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourceAttributes(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResourceAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if got != nil {
					t.Errorf("Expected nil result on error, got %v", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("ParseResourceAttributes() got %d items, want %d", len(got), len(tt.want))
			}

			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseResourceAttributes() key %q = %v, want %v", k, got[k], v)
				}
			}

			// Test round-trip: stringify and parse again should give same result
			if len(tt.want) > 0 {
				str := got.String()
				got2, err2 := ParseResourceAttributes(str)
				if err2 != nil {
					t.Fatalf("Round-trip parsing failed: %v", err2)
				}

				if len(got2) != len(tt.want) {
					t.Errorf("Round-trip parsing got %d items, want %d", len(got2), len(tt.want))
				}

				for k, v := range tt.want {
					if got2[k] != v {
						t.Errorf("Round-trip parsing key %q = %v, want %v", k, got2[k], v)
					}
				}
			}
		})
	}
}

func TestResourceAttributes_String(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  string
	}{
		{
			name:  "nil map",
			input: nil,
			want:  "",
		},
		{
			name:  "empty map",
			input: map[string]string{},
			want:  "",
		},
		{
			name:  "single pair",
			input: map[string]string{"service.name": "api"},
			want:  "service.name=api",
		},
		{
			name:  "sorts keys deterministically",
			input: map[string]string{"b": "2", "a": "1"},
			want:  "a=1,b=2",
		},
		{
			name: "escapes commas equals and backslashes in values",
			input: map[string]string{
				"team":  "platform,core",
				"token": "a=b",
				"path":  `C:\apps\api`,
			},
			want: `path=C:\\apps\\api,team=platform\,core,token=a\=b`,
		},
		{
			name: "escapes commas equals and backslashes in keys",
			input: map[string]string{
				`weird,key`:  "v1",
				`weird=key2`: "v2",
				`weird\key3`: "v3",
			},
			want: `weird\,key=v1,weird\=key2=v2,weird\\key3=v3`,
		},
		{
			name: "skips empty/whitespace keys defensively",
			input: map[string]string{
				"":     "x",
				"   ":  "y",
				"good": "z",
			},
			want: "good=z",
		},
		{
			name:  "empty value preserved",
			input: map[string]string{"a": ""},
			want:  "a=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceAttributes(tt.input).String()
			if got != tt.want {
				t.Fatalf("unexpected serialization\n got:  %q\n want: %q", got, tt.want)
			}
		})
	}
}
