package main

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

// msg_format constructs a key-value string using predefined separators
func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}
