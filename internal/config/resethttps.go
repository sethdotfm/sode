package config

// For the -reset-https console flag. A missing file is not an error.
func ClearRequireHTTPS(path string) (was bool, err error) {
	c, exists, err := Load(path)
	if err != nil || !exists || !c.RequireHTTPS {
		return false, err
	}
	c.RequireHTTPS = false
	return true, Save(path, c)
}
