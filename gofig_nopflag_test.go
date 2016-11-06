// +build !pflag

package gofig

func testReg3() *configReg {
	r := newRegistration("Mock Provider")
	r.SetYAML(`mockProvider:
    userName: admin
    password: ""
    insecure: true
    useCerts: true
    docker:
        MinVolSize: 16
        maxVolSize: 256
`)
	r.Key(String, "", "admin", "", "mockProvider.userName")
	r.Key(String, "", "", "", "mockProvider.password")
	r.Key(Bool, "", false, "", "mockProvider.useCerts")
	r.Key(Int, "", 16, "", "mockProvider.docker.minVolSize")
	r.Key(Bool, "i", true, "", "mockProvider.insecure")
	r.Key(Int, "m", 256, "", "mockProvider.docker.maxVolSize")
	return r
}
