package rbhparser

import "github.com/ethereum/go-ethereum/common"

type AddressBook struct {
	PoolManager       common.Address
	UniversalRouter   common.Address
	WETH              common.Address
	PonsFactory       common.Address
	PonsLaunchAndBuy  common.Address
	PonsMemeHook      common.Address
	LongLauncher      common.Address
	O1Factory         common.Address
	O1Hook            common.Address
	PoolsEntry        common.Address
	PoolsTokenFactory common.Address
	PAIRLaunchpad     common.Address
	BagsFactory       common.Address
	BagsHook          common.Address
}

func DefaultAddressBook() AddressBook {
	return AddressBook{
		PoolManager:       common.HexToAddress("0x8366a39CC670B4001A1121B8F6A443A643e40951"),
		UniversalRouter:   common.HexToAddress("0x8876789976dEcBfCbBbe364623C63652db8C0904"),
		WETH:              common.HexToAddress("0x0Bd7D308f8E1639FAb988df18A8011F41EAcAD73"),
		PonsFactory:       common.HexToAddress("0x7eD598BcEf8bd9Edd8C97A195C6d13f40801EC7e"),
		PonsLaunchAndBuy:  common.HexToAddress("0xe33E9E479dF8802cb0866d5d05258bEc4cF62948"),
		PonsMemeHook:      common.HexToAddress("0xE5e702641Ea86F4ae6cC3cDaeD2B886f976Be044"),
		LongLauncher:      common.HexToAddress("0x22e99278308B393ea1260859B181AD7E78f5eeED"),
		O1Factory:         common.HexToAddress("0xcE9C48cFa068947f77738c81Be406B53338E5B0d"),
		O1Hook:            common.HexToAddress("0x0310cFEbE1D7A69f2414f6595bBe9d17c5342aCc"),
		PoolsEntry:        common.HexToAddress("0x0000FffFBE8efE702c8703aE3477FF5dE3d319C0"),
		PoolsTokenFactory: common.HexToAddress("0x000000e200088D55C39a11F609E5F667729ad49b"),
		PAIRLaunchpad:     common.HexToAddress("0x8660A7F019C7943b0b0A91B8E39AFf3b6DB6Ae62"),
		BagsFactory:       common.HexToAddress("0xe8Cc4431adF8b5A847C113EF0c6af9043219Cb37"),
		BagsHook:          common.HexToAddress("0x2380aBf72C17aABAb76480244759AC7E2932EEcC"),
	}
}
