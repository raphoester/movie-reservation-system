package xconfigs

import "io/fs"

func WithSecretManagers(sms ...SecretManager) LoadOption {
	return func(o loadParams) loadParams {
		o.secretManagers = sms
		return o
	}
}

func WithSharedConfigFS(fsys fs.FS) LoadOption {
	return func(o loadParams) loadParams {
		o.sharedConfigFS = fsys
		return o
	}
}
