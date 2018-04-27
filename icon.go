package external

func IconURL(assetId string) string {
	switch assetId {
	case BitcoinChainId:
		return "https://images.mixin.one/HvYGJsV5TGeZ-X9Ek3FEQohQZ3fE9LBEBGcOcn4c4BNHovP4fW4YB97Dg5LcXoQ1hUjMEgjbl1DPlKg1TW7kK6XP=s128"
	case BitcoinCashChainId:
		return "https://images.mixin.one/tqt14x8iwkiCR_vIKIw6gAAVO8XpZH7ku7ZJYB5ArMRA6grN9M1oCI7kKt2QqBODJwr17sZxDCDTjXHOgIixzv6X=s128"
	case LitecoinChainId:
		return "https://images.mixin.one/EoQc9afnnbH9nu5mBa7IUYpow-lXbnD7eoLgwdtKgJzt21dZ_kSwQVyzALxhqDqe-yN9M75KEzstbtAtXylxctLG=s128"
	case RippleChainId:
		return "https://images.mixin.one/zgBBzgQ8x2XZbNdRhibMlVLyG0x1PEr8ENepLODl_mBDIXJJSCZNoaa2jHCc4VpJ59hPgUVnL6W24v1utEVqb0A=s128"
	case EthereumClassicChainId:
		return "https://images.mixin.one/fM9wgyNyB3Uiopx2FRFxhr-sYrvXZtJ-uCpk975wGdpoehoA59rIU-BQ4s_6YFMDEthQ74KCPysOIWSFK4vUG_Y=s128"
	case EthereumChainId:
		return "https://images.mixin.one/zVDjOxNTQvVsA8h2B4ZVxuHoCF3DJszufYKWpd9duXUSbSapoZadC7_13cnWBqg0EmwmRcKGbJaUpA8wFfpgZA=s128"
	case UniqueAssetId(EthereumChainId, EthereumXINAssetKey):
		return "https://images.mixin.one/E2y0BnTopFK9qey0YI-8xV3M82kudNnTaGw0U5SU065864SsewNUo6fe9kDF1HIzVYhXqzws4lBZnLj1lPsjk-0=s128"
	case UniqueAssetId(EthereumChainId, EthereumEOSAssetKey):
		return "https://images.mixin.one/a5dtG-IAg2IO0Zm4HxqJoQjfz-5nf1HWZ0teCyOnReMd3pmB8oEdSAXWvFHt2AJkJj5YgfyceTACjGmXnI-VyRo=s128"
	case UniqueAssetId(EthereumChainId, EthereumBIGAssetKey):
		return "https://images.mixin.one/5YTXC-s9OFwY5KihNODO3o9lZfHT3XQvfBgHbneV6JfR4JG50pmeG9mW51n5HftfjH6krqJTsQCAWYakF-Cil5A=s128"
	case UniqueAssetId(EthereumChainId, EthereumPRSAssetKey):
		return "https://images.mixin.one/1fQiAdit_Ji6_Pf4tW8uzutONh9kurHhAnN4wqEIItkDAvFTSXTMwlk3AB749keufDFVoqJb5fSbgz7K2HoOV7Q=s128"
	case UniqueAssetId(EthereumChainId, EthereumZRXAssetKey):
		return "https://images.mixin.one/XnuEAvz7ErxgS9XQi2chew601g5Y6CNv3v-SjPlIKKTa3FTFVodVXL9Eqhj1PRNGLpTzJ5qiGtG-XWxLDbBStg=s128"
	case UniqueAssetId(EthereumChainId, EthereumSNTAssetKey):
		return "https://images.mixin.one/lXBeROLo7DWiVpVtJ0j4wGd56HIM-yBNXXAKwSr4A60i9-zKcK_Xo4YYbv2rT2mKBqkq8yTgU5gu1e9_ZlM5Rgce=s128"
	case UniqueAssetId(EthereumChainId, EthereumOMGAssetKey):
		return "https://images.mixin.one/mZXzWZ3tRhWCuQfTIGEjKE7IwBXZXoTJE1N5hsgiQ280cd2ARA6jqPrTryaC_K0xmvRgijBfhMRjY8ijO82oVg=s128"
	case UniqueAssetId(EthereumChainId, EthereumDEWAssetKey):
		return "https://images.mixin.one/M48NX5cG8o7aDlzFly_U--n211kRmOnOmNjMsIGAHbZHgyXs2ZQz4oMV3cjTtMbU6pNfzzC-rV1c4wrT8EOWdz0Y=s128"
	case UniqueAssetId(EthereumChainId, EthereumDGDAssetKey):
		return "https://images.mixin.one/8LahZDeQCDIJH2GUxvgf6OXc_mzjCZfb0tilJEtbfkm_Ltlu32KvHNiYI-uNWoBvAlFz_w3p3NzsVURV08Oraozb=s128"
	case UniqueAssetId(EthereumChainId, EthereumBATAssetKey):
		return "https://images.mixin.one/19UpwbJVwanBqr8uCtVxcxoujYCFPo15KUeWc4KhyU7Yt_FHcsHAvmiuo3udTX1NvLZVAa-g06sTAt9TlcrNsg=s128"
	case UniqueAssetId(EthereumChainId, EthereumCANDYAssetKey):
		return "https://images.mixin.one/wrGUPXYeb4zw1cm9KL27ftmqEWdu2-ltzKQlRAlHe4ulneHj7IdTwgPdsX4csuxYBAuIi8_iBfMI7yX6xQmlQA=s128"
	case UniqueAssetId(EthereumChainId, EthereumTRXAssetKey):
		return "https://images.mixin.one/7FL7yfE6Ca3RRew1GiWEHbOz2nkJ-n12RwxeZ7Pi_KVX9d8Ux8YX-_B7cWIpBczT1Jw2OjY8fo-P4CxbiZ85yDE=s128"
	case UniqueAssetId(EthereumChainId, EthereumVENAssetKey):
		return "https://images.mixin.one/GP9crdLFtZkp8Qvqu93r4XrsYqgxDmte-qdpdgw__XbQ5h8V3vi7OJz1aCNYpX6rgturRP3XYateJTdtac1yNQU=s128"
	case UniqueAssetId(EthereumChainId, EthereumCVCAssetKey):
		return "https://images.mixin.one/InpT6RTK-tFLV7oUbCuG0J17vCNbRHW8iG_ZzPAkvCTZrtMaTXUnJ7VsYEDBlPFxddu9wvb5ctzmrawXcFsCPSo=s128"
	case UniqueAssetId(EthereumChainId, EthereumCNBAssetKey):
		return "https://images.mixin.one/0sQY63dDMkWTURkJVjowWY6Le4ICjAFuu3ANVyZA4uI3UdkbuOT5fjJUT82ArNYmZvVcxDXyNjxoOv0TAYbQTNKS=s128"
	case UniqueAssetId(EthereumChainId, EthereumIOSTAssetKey):
		return "https://images.mixin.one/lKbTmzs1fOAjxYYWrENtdCZINkgefEEohsHc-PQePGYVDkx2K_HVYa4PjbwDhXHPp9U8RDzA1g0rNRlbtr0b2q4=s128"
	case UniqueAssetId(EthereumChainId, EthereumRUFFAssetKey):
		return "https://images.mixin.one/YSwenktF4DbUYUq6L5X73eW_c9QjVmLKiCS1eai4gY51ar9Uf0A8OD1VVLTgC3vntpGCWIXFR9UITQXIZCIfILE=s128"
	case UniqueAssetId(EthereumChainId, EthereumIHTAssetKey):
		return "https://images.mixin.one/giY3a2sgy7VYuzBjJRqZEkknP3pXRRo6a90-Pt1YNh4-7SmhJ9Xm73GuwkN1m9P71wvuPvXyU3F7KpLWttR4b6c=s128"
	case UniqueAssetId(EthereumChainId, EthereumDTAAssetKey):
		return "https://images.mixin.one/DefUNBLrmqoFkICcvFNDIUUNIHY0oJbngIZKkFH9Sx7TbjojOijWSeLNXARZDjRF2qfzv2DQpVVXZDN0vlR5LiU=s128"
	case UniqueAssetId(EthereumChainId, EthereumTNBAssetKey):
		return "https://images.mixin.one/AurNTJNyD1Z1au4wbpK9HPVmXDvE37eldvLR5hHRMVxHxn4DxiQ1CWonW82MbCaOoBvBsNnJfb5aC_33ueuMFxs=s128"
	case UniqueAssetId(EthereumChainId, EthereumLRCAssetKey):
		return "https://images.mixin.one/qVagXT8U1OgPbJ1Sd45k2-15z69BRpVaTrca39Bei7r85rv3ROBIPVvbuuCSs2LQKkdKBGcVXoPD9yfQJdIWaw=s128"
	case UniqueAssetId(EthereumChainId, EthereumnCashAssetKey):
		return "https://images.mixin.one/-17id08Az75h3J6l7wKe7SE15iVgudZjVMa8T1AdpXzQIuAqpwDZphvwCLbpHe4xr8CYEnaRDaK_hw7tD3Pplw=s128"
	case UniqueAssetId(EthereumChainId, EthereumnCREAssetKey):
		return "https://images.mixin.one/KYbiNh2H8Z5L5bRCcpy9UwjU7CZ7l17PSNFkFbkpjHhuOx1hpzcUoHrBC6oNy9-QHUJrmCvau4MOi8B9UFRBNnn9=s128"
	case UniqueAssetId(EthereumChainId, EthereumnQUNAssetKey):
		return "https://images.mixin.one/Ij_kVcR5X2xINYyezPYJqy56_52CZqVr55e5-LBSi4sO27exwYVWY6YxQIak9vUCkZYFoA2uPVMuKF6XwlWM4XU=s128"
	case UniqueAssetId(EthereumChainId, EthereumnPAYAssetKey):
		return "https://images.mixin.one/DYS3R3lIODC6bodKQSXDBWuqs_tR12ch7k-1NsFRiyhCAp13JS-PUAmHE0QoNpoi-p4DDzQASKiJvlEvUb5VlqAJ=s128"
	case UniqueAssetId(EthereumChainId, EthereumnUSDTAssetKey):
		return "https://images.mixin.one/ndNBEpObYs7450U08oAOMnSEPzN66SL8Mh-f2pPWBDeWaKbXTPUIdrZph7yj8Z93Rl8uZ16m7Qjz-E-9JFKSsJ-F=s128"
	}
	return "https://images.mixin.one/HovctUnrBkLPlDotWvWPsIuFb8qKrLddwF5-f2Fi9q9uO829YB2qGITgOd2YmTMKnGg_z9XrVYzEwFE_rD_REz9C=s128"
}
