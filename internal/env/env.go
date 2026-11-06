package env

const Undefined = "undefined"

var (
	Title = "DarkDMM"
	// AppDir is a name of the folder with settings, logs and backups. Every build variant has its own.
	AppDir   = "DarkDMM"
	Version  = "2.18-dark7"
	Revision = Undefined
	GitHub   = "https://github.com/AndreeykasTea/DarkDMM"
	// Manifest is empty: self updates are disabled in DarkDMM, see app/update.go.
	Manifest = ""
	// Support is the original author of StrongDMM, DarkDMM has no donations.
	Support = "https://ko-fi.com/spair"
)
