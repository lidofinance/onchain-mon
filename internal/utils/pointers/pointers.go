package pointers

// Helper function to create pointers for strings
func StringPtr(s string) *string {
	return &s
}

// Helper function to create pointers for ints
func IntPtr(i int) *int {
	return &i
}
