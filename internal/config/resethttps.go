package config

// ClearRequireHTTPS turns Require HTTPS off in the file at path, for the -reset-https
// console flag. It reports whether the setting was on. A missing file is not an error.
func ClearRequireHTTPS(path string) (was bool, err error) {
	c, exists, err := Load(path)
	if err != nil || !exists || !c.RequireHTTPS {
		return false, err
	}
	c.RequireHTTPS = false
	return true, Save(path, c)
}
