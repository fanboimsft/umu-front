package main

type Game struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ExecPath  string `json:"exec_path"`
	Prefix       string `json:"prefix"`
	ProtonVer    string `json:"proton_ver"`
	ImageURL     string `json:"image_url"`
	DLLOverrides string `json:"dll_overrides,omitempty"`
}
