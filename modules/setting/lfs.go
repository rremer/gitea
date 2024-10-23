// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/generate"
)

const (
	LFSConfigSectionLegacyServer = "server"
	LFSConfigSectionServer       = "lfs.server"
	LFSConfigSectionClient       = "lfs.client"
)

// LFS represents the legacy configuration for Git LFS, to be migrated to LFSServer
var LFS = struct {
	StartServer    bool          `ini:"LFS_START_SERVER"`
	AllowPureSSH   bool          `ini:"LFS_ALLOW_PURE_SSH"`
	JWTSecretBytes []byte        `ini:"-"`
	HTTPAuthExpiry time.Duration `ini:"LFS_HTTP_AUTH_EXPIRY"`
	MaxFileSize    int64         `ini:"LFS_MAX_FILE_SIZE"`
	LocksPagingNum int           `ini:"LFS_LOCKS_PAGING_NUM"`

	Storage *Storage
}{}

// LFSServer represents configuration for hosting Git LFS
var LFSServer = struct {
	MaxBatchSize int `ini:"MAX_BATCH_SIZE"`
}{}

// LFSClient represents configuration for mirroring upstream Git LFS
var LFSClient = struct {
	BatchSize int `ini:"BATCH_SIZE"`
}{}

func loadLFSFrom(rootCfg ConfigProvider) error {
	mustMapSetting(rootCfg, LFSConfigSectionServer, &LFSServer)
	mustMapSetting(rootCfg, LFSConfigSectionClient, &LFSClient)
	mustMapSetting(rootCfg, LFSConfigSectionLegacyServer, &LFS)

	legacySec := rootCfg.Section(LFSConfigSectionLegacyServer)

	lfsSec, _ := rootCfg.GetSection("lfs")

	// Specifically default PATH to LFS_CONTENT_PATH
	// DEPRECATED should not be removed because users maybe upgrade from lower version to the latest version
	// if these are removed, the warning will not be shown
	deprecatedSetting(rootCfg, "server", "LFS_CONTENT_PATH", "lfs", "PATH", "v1.19.0")

	if val := legacySec.Key("LFS_CONTENT_PATH").String(); val != "" {
		if lfsSec == nil {
			lfsSec = rootCfg.Section("lfs")
		}
		lfsSec.Key("PATH").MustString(val)
	}

	var err error
	LFS.Storage, err = getStorage(rootCfg, "lfs", "", lfsSec)
	if err != nil {
		return err
	}

	// Rest of LFS service settings
	if LFS.LocksPagingNum == 0 {
		LFS.LocksPagingNum = 50
	}

	if LFSClient.BatchSize < 1 {
		LFSClient.BatchSize = 20
	}

	LFS.HTTPAuthExpiry = legacySec.Key("LFS_HTTP_AUTH_EXPIRY").MustDuration(24 * time.Hour)

	if !LFS.StartServer || !InstallLock {
		return nil
	}

	jwtSecretBase64 := loadSecret(rootCfg.Section("server"), "LFS_JWT_SECRET_URI", "LFS_JWT_SECRET")
	LFS.JWTSecretBytes, err = generate.DecodeJwtSecretBase64(jwtSecretBase64)
	if err != nil {
		LFS.JWTSecretBytes, jwtSecretBase64, err = generate.NewJwtSecretWithBase64()
		if err != nil {
			return fmt.Errorf("error generating JWT Secret for custom config: %v", err)
		}

		// Save secret
		saveCfg, err := rootCfg.PrepareSaving()
		if err != nil {
			return fmt.Errorf("error saving JWT Secret for custom config: %v", err)
		}
		rootCfg.Section("server").Key("LFS_JWT_SECRET").SetValue(jwtSecretBase64)
		saveCfg.Section("server").Key("LFS_JWT_SECRET").SetValue(jwtSecretBase64)
		if err := saveCfg.Save(); err != nil {
			return fmt.Errorf("error saving JWT Secret for custom config: %v", err)
		}
	}

	return nil
}
