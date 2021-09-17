package doom

const VERSION = 1
const SUBVERSION = 3

var application = "SRB2Kart"

// Create a string of length MAXAPPLICATION for use in client config checks
var SRB2APPLICATION = append([]byte(application), make([]byte, MAXAPPLICATION-len(application))...)
