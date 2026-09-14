package switchfs

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
)

type Language int

const (
	AmericanEnglish = iota
	BritishEnglish
	Japanese
	French
	German
	LatinAmericanSpanish
	Spanish
	Italian
	Dutch
	CanadianFrench
	Portuguese
	Russian
	Korean
	Taiwanese
	Chinese
)

type NacpTitle struct {
	Language Language
	Title    string
}

type Nacp struct {
	TitleName             map[string]NacpTitle
	Isbn                  string
	DisplayVersion        string
	SupportedLanguageFlag uint32
}

func (l Language) String() string {
	names := [...]string{
		"AmericanEnglish",
		"BritishEnglish",
		"Japanese",
		"French",
		"German",
		"LatinAmericanSpanish",
		"Spanish",
		"Italian",
		"Dutch",
		"CanadianFrench",
		"Portuguese",
		"Russian",
		"Korean",
		"Taiwanese",
		"Chinese",
		"Chinese"}
	if l < 0 || int(l) >= len(names) {
		return "Unknown"
	}
	return names[l]
}

func ExtractNacp(cnmt *ContentMetaAttributes, file io.ReaderAt, securePartition *PFS0, securePartitionOffset int64) (*Nacp, error) {
	if control, ok := cnmt.Contents["Control"]; ok {
		controlNca := getNcaById(securePartition, control.ID)
		if controlNca != nil {
			fsHeader, section, err := openMetaNcaDataSection(file, securePartitionOffset+int64(controlNca.StartOffset))
			if err != nil {
				return nil, err
			}
			if fsHeader.fsType == 0 {
				romFsHeader, err := readRomfsHeader(section)
				if err != nil {
					return nil, err
				}
				fEntries, err := readRomfsFileEntry(section, romFsHeader)
				if err != nil {
					return nil, err
				}

				if entry, ok := fEntries["control.nacp"]; ok {
					nacp, err := readNacp(section, romFsHeader, entry)
					if err != nil {
						return nil, err
					}
					return &nacp, nil
				}
			} else {
				return nil, errors.New("unsupported type " + control.ID)
			}
		} else {
			return nil, errors.New("unable to find control.nacp by id " + control.ID)
		}

	}
	return nil, errors.New("no control.nacp found")
}

/*https://switchbrew.org/wiki/NACP_Format*/
func readNacp(data []byte, romFsHeader RomfsHeader, fileEntry RomfsFileEntry) (Nacp, error) {
	if romFsHeader.DataOffset > uint64(len(data)) || fileEntry.offset > uint64(len(data))-romFsHeader.DataOffset {
		return Nacp{}, errors.New("NACP file is outside RomFS data")
	}
	offset := romFsHeader.DataOffset + fileEntry.offset
	if fileEntry.size < 0x3080 {
		return Nacp{}, errors.New("NACP file is too small")
	}
	if fileEntry.size > uint64(len(data))-offset || uint64(0x3080) > fileEntry.size {
		return Nacp{}, errors.New("NACP file is truncated")
	}
	titleData := data[offset : offset+0x3000]
	titleStride := uint64(0x300)
	if fileEntry.size >= 0x3216 && data[offset+0x3215] == 1 {
		compressedSize := binary.LittleEndian.Uint16(titleData[:2])
		if compressedSize == 0 || uint64(compressedSize) > uint64(len(titleData)-2) {
			return Nacp{}, errors.New("compressed NACP title data is invalid")
		}
		reader := flate.NewReader(bytes.NewReader(titleData[2 : 2+compressedSize]))
		decompressed, err := io.ReadAll(io.LimitReader(reader, 0x6001))
		closeErr := reader.Close()
		if err != nil {
			return Nacp{}, errors.New("failed to decompress NACP title data: " + err.Error())
		}
		if closeErr != nil {
			return Nacp{}, errors.New("failed to close NACP title data: " + closeErr.Error())
		}
		if len(decompressed) < 0x6000 {
			return Nacp{}, errors.New("decompressed NACP title data is too small")
		}
		if len(decompressed) > 0x6000 {
			return Nacp{}, errors.New("decompressed NACP title data is too large")
		}
		titleData = decompressed[:0x6000]
		titleStride = 0x600
	}
	titles := map[string]NacpTitle{}
	for i := 0; i < 16; i++ {
		//lang := i
		appTitleOffset := uint64(i) * titleStride
		appTitleBytes := titleData[appTitleOffset : appTitleOffset+0x200]
		nameBytes := readBytesUntilZero(appTitleBytes)
		titles[Language(i).String()] = NacpTitle{Language: Language(i), Title: string(nameBytes)}
	}

	isbn := readBytesUntilZero(data[offset+0x3000 : offset+0x3000+0x25])
	displayVersion := readBytesUntilZero(data[offset+0x3060 : offset+0x3060+0x10])
	supportedLanguageFlag := binary.LittleEndian.Uint32(data[offset+0x302C : offset+0x302C+0x4])

	return Nacp{TitleName: titles, Isbn: string(isbn), DisplayVersion: string(displayVersion), SupportedLanguageFlag: supportedLanguageFlag}, nil
	/*


	   Isbn = reader.ReadUtf8Z(37);
	   reader.BaseStream.Position = start + 0x3025;
	   StartupUserAccount = reader.ReadByte();
	   UserAccountSwitchLock = reader.ReadByte();
	   AocRegistrationType = reader.ReadByte();
	   AttributeFlag = reader.ReadInt32();
	   supportedLanguageFlag = reader.ReadUInt32();
	   ParentalControlFlag = reader.ReadUInt32();
	   Screenshot = reader.ReadByte();
	   VideoCapture = reader.ReadByte();
	   DataLossConfirmation = reader.ReadByte();
	   PlayLogPolicy = reader.ReadByte();
	   PresenceGroupId = reader.ReadUInt64();

	   for (int i = 0; i < RatingAge.Length; i++)
	   {
	       RatingAge[i] = reader.ReadSByte();
	   }

	   DisplayVersion = reader.ReadUtf8Z(16);
	   reader.BaseStream.Position = start + 0x3070;
	   AddOnContentBaseId = reader.ReadUInt64();
	   SaveDataOwnerId = reader.ReadUInt64();
	   UserAccountSaveDataSize = reader.ReadInt64();
	   UserAccountSaveDataJournalSize = reader.ReadInt64();
	   DeviceSaveDataSize = reader.ReadInt64();
	   DeviceSaveDataJournalSize = reader.ReadInt64();
	   BcatDeliveryCacheStorageSize = reader.ReadInt64();
	   ApplicationErrorCodeCategory = reader.ReadUtf8Z(8);
	   reader.BaseStream.Position = start + 0x30B0;

	   for (int i = 0; i < LocalCommunicationId.Length; i++)
	   {
	       LocalCommunicationId[i] = reader.ReadUInt64();
	   }

	   LogoType = reader.ReadByte();
	   LogoHandling = reader.ReadByte();
	   RuntimeAddOnContentInstall = reader.ReadByte();
	   Reserved00 = reader.ReadBytes(3);
	   CrashReport = reader.ReadByte();
	   Hdcp = reader.ReadByte();
	   SeedForPseudoDeviceId = reader.ReadUInt64();
	   BcatPassphrase = reader.ReadUtf8Z(65);

	   reader.BaseStream.Position = start + 0x3141;
	   Reserved01 = reader.ReadByte();
	   Reserved02 = reader.ReadBytes(6);

	   UserAccountSaveDataSizeMax = reader.ReadInt64();
	   UserAccountSaveDataJournalSizeMax = reader.ReadInt64();
	   DeviceSaveDataSizeMax = reader.ReadInt64();
	   DeviceSaveDataJournalSizeMax = reader.ReadInt64();
	   TemporaryStorageSize = reader.ReadInt64();
	   CacheStorageSize = reader.ReadInt64();
	   CacheStorageJournalSize = reader.ReadInt64();
	   CacheStorageDataAndJournalSizeMax = reader.ReadInt64();
	   CacheStorageIndex = reader.ReadInt16();
	   Reserved03 = reader.ReadBytes(6);

	   for (int i = 0; i < 16; i++)
	   {
	       ulong value = reader.ReadUInt64();
	       if (value != 0) PlayLogQueryableApplicationId.Add(value);
	   }

	   PlayLogQueryCapability = reader.ReadByte();
	   RepairFlag = reader.ReadByte();
	   ProgramIndex = reader.ReadByte();

	   UserTotalSaveDataSize = UserAccountSaveDataSize + UserAccountSaveDataJournalSize;
	   DeviceTotalSaveDataSize = DeviceSaveDataSize + DeviceSaveDataJournalSize;
	   TotalSaveDataSize = UserTotalSaveDataSize + DeviceTotalSaveDataSize;
	*/
}

func readBytesUntilZero(appTitleBytes []byte) []byte {
	var nameBytes []byte
	for _, b := range appTitleBytes {
		if b == 0x0 {
			break
		} else {
			nameBytes = append(nameBytes, b)
		}
	}
	return nameBytes
}
