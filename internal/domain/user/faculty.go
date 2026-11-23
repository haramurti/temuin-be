package user

// Faculty is one UB faculty option for the Sign Up dropdown and pos satpam list.
type Faculty struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Faculties lists Universitas Brawijaya faculties (stored on the user as Name).
var Faculties = []Faculty{
	{"FH", "Fakultas Hukum"},
	{"FEB", "Fakultas Ekonomi dan Bisnis"},
	{"FIA", "Fakultas Ilmu Administrasi"},
	{"FP", "Fakultas Pertanian"},
	{"FAPET", "Fakultas Peternakan"},
	{"FT", "Fakultas Teknik"},
	{"FK", "Fakultas Kedokteran"},
	{"FPIK", "Fakultas Perikanan dan Ilmu Kelautan"},
	{"FMIPA", "Fakultas Matematika dan Ilmu Pengetahuan Alam"},
	{"FTP", "Fakultas Teknologi Pertanian"},
	{"FISIP", "Fakultas Ilmu Sosial dan Ilmu Politik"},
	{"FIB", "Fakultas Ilmu Budaya"},
	{"FKH", "Fakultas Kedokteran Hewan"},
	{"FILKOM", "Fakultas Ilmu Komputer"},
	{"FIKES", "Fakultas Ilmu Kesehatan"},
	{"FKG", "Fakultas Kedokteran Gigi"},
	{"VOKASI", "Fakultas Vokasi"},
}

// NormalizeFaculty accepts a code or full name and returns the canonical name.
func NormalizeFaculty(v string) (string, bool) {
	for _, f := range Faculties {
		if equalFold(v, f.Code) || equalFold(v, f.Name) {
			return f.Name, true
		}
	}
	return "", false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 32
		}
		if 'A' <= y && y <= 'Z' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}
