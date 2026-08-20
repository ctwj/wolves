package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeka/zip"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// RepackageResult 重新打包结果
type RepackageResult struct {
	Data          []byte
	DeletedCount  int
	KeptCount     int
	AddedCount    int
	AddErrors     []string // 添加文件失败的错误信息
	SkippedFiles  []string // 因错误跳过的原始文件
}

// safeOpenFile 安全打开 ZIP 文件，捕获可能的 panic
func safeOpenFile(file *zip.File) (rc io.ReadCloser, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic when opening zip file %s: %v", file.Name, r)
			zap.S().Warnf("打开 ZIP 文件发生 panic: %s, 错误: %v", file.Name, r)
		}
	}()
	rc, err = file.Open()
	return
}

// RepackageZip 重新打包 ZIP 文件
// originalData: 原始 ZIP 数据
// deleteFiles: 要删除的文件列表（支持模糊匹配，如 "广告" 会删除所有包含 "广告" 的文件）
// addFiles: 要添加的本地文件列表（需要绝对路径）
// renameRules: 文件名重命名规则（旧名: 新名）
// password: 压缩包密码，为空则不加密
// noEncryptFiles: 不需要加密的文件列表（支持模糊匹配），为空则全部加密
func RepackageZip(originalData []byte, deleteFiles, addFiles, renameRules []string, password string, noEncryptFiles []string) (*RepackageResult, error) {
	// 读取原始 ZIP
	reader, err := zip.NewReader(bytes.NewReader(originalData), int64(len(originalData)))
	if err != nil {
		return nil, err
	}

	// 创建新的 ZIP buffer
	buf := new(bytes.Buffer)
	writer := zip.NewWriter(buf)

	result := &RepackageResult{}

	zap.S().Debugf("开始重新打包 ZIP，删除规则: %v, 添加文件: %v, 密码: %v", deleteFiles, addFiles, password != "")

	// 复制原始文件（跳过要删除的）
	for _, file := range reader.File {
		// 检查是否需要删除（支持模糊匹配）
		if shouldDeleteFile(file.Name, deleteFiles) {
			result.DeletedCount++
			zap.S().Infof("删除 ZIP 文件: %s", file.Name)
			continue
		}

		result.KeptCount++

		// 应用重命名规则
		newName := applyRenameRules(file.Name, renameRules)

		// 打开原文件（使用安全方式，捕获可能的 panic）
		src, err := safeOpenFile(file)
		if err != nil {
			// 记录错误，跳过该文件而不是整个流程失败
			result.SkippedFiles = append(result.SkippedFiles, file.Name)
			zap.S().Warnf("打开 ZIP 文件失败，跳过: %s, 错误: %v", file.Name, err)
			result.KeptCount-- // 调整计数
			continue
		}

		// 创建新文件
		var dst io.Writer
		// 判断是否需要加密：有密码且不在不加密列表中
		shouldEncrypt := password != "" && !shouldNotEncrypt(newName, noEncryptFiles)
		if shouldEncrypt {
			// 使用 StandardEncryption (ZipCrypto)，兼容性更好
			dst, err = writer.Encrypt(newName, password, zip.StandardEncryption)
			if err != nil {
				src.Close()
				return nil, err
			}
		} else {
			// 使用 FileHeader 创建文件，设置 UTF-8 标志解决中文乱码
			header := &zip.FileHeader{
				Name:   newName,
				Flags:  0x800, // UTF-8 标志位
				Method: zip.Deflate,
			}
			dst, err = writer.CreateHeader(header)
			if err != nil {
				src.Close()
				return nil, err
			}
		}

		// 复制内容
		_, err = io.Copy(dst, src)
		src.Close()
		if err != nil {
			return nil, err
		}
	}

	// 添加新文件（失败时记录警告，不中断流程）
	for _, filePath := range addFiles {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}
		// 读取本地文件
		data, err := os.ReadFile(filePath)
		if err != nil {
			errMsg := "添加文件失败: " + filePath + " - " + err.Error()
			result.AddErrors = append(result.AddErrors, errMsg)
			zap.S().Warnf("添加文件到 ZIP 失败: %s, 错误: %v", filePath, err)
			continue
		}

		// 添加到 ZIP
		var dst io.Writer
		// 判断是否需要加密：有密码且不在不加密列表中
		shouldEncrypt := password != "" && !shouldNotEncrypt(filepath.Base(filePath), noEncryptFiles)
		if shouldEncrypt {
			// 使用 StandardEncryption (ZipCrypto)，兼容性更好
			dst, err = writer.Encrypt(filepath.Base(filePath), password, zip.StandardEncryption)
			if err != nil {
				result.AddErrors = append(result.AddErrors, "加密失败: "+filePath)
				zap.S().Warnf("加密添加的文件失败: %s, 错误: %v", filePath, err)
				continue
			}
		} else {
			// 使用 FileHeader 创建文件，设置 UTF-8 标志解决中文乱码
			header := &zip.FileHeader{
				Name:   filepath.Base(filePath),
				Flags:  0x800, // UTF-8 标志位
				Method: zip.Deflate,
			}
			dst, err = writer.CreateHeader(header)
			if err != nil {
				result.AddErrors = append(result.AddErrors, "创建文件头失败: "+filePath)
				zap.S().Warnf("创建 ZIP 文件头失败: %s, 错误: %v", filePath, err)
				continue
			}
		}

		if _, err := dst.Write(data); err != nil {
			result.AddErrors = append(result.AddErrors, "写入文件内容失败: "+filePath)
			zap.S().Warnf("写入文件内容到 ZIP 失败: %s, 错误: %v", filePath, err)
			continue
		}

		result.AddedCount++
		zap.S().Infof("添加文件到 ZIP: %s", filePath)
	}

	// 关闭 writer
	if err := writer.Close(); err != nil {
		return nil, err
	}

	// 返回结果
	result.Data = buf.Bytes()
	zap.S().Infof("ZIP 重新打包完成: 删除 %d 个文件, 保留 %d 个文件, 添加 %d 个文件", result.DeletedCount, result.KeptCount, result.AddedCount)
	return result, nil
}

// decodeGBKFilename 尝试将 GBK 编码的文件名解码为 UTF-8
func decodeGBKFilename(filename string) string {
	// 尝试从 GBK 解码
	reader := transform.NewReader(strings.NewReader(filename), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return filename
	}
	return string(decoded)
}

// shouldDeleteFile 检查文件是否应该被删除（支持模糊匹配，支持 GBK 编码文件名）
func shouldDeleteFile(filename string, deleteFiles []string) bool {
	// 获取 UTF-8 解码后的文件名（可能是 GBK 编码）
	decodedFilename := decodeGBKFilename(filename)
	
	zap.S().Debugf("检查文件是否删除: filename=%s, decoded=%s, deleteFiles=%v", filename, decodedFilename, deleteFiles)
	
	for _, pattern := range deleteFiles {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		// 完全匹配（尝试原始文件名和解码后的文件名）
		if filename == pattern || decodedFilename == pattern {
			zap.S().Infof("文件完全匹配删除: %s (decoded: %s)", filename, decodedFilename)
			return true
		}
		// 模糊匹配：文件名包含指定字符串（尝试两种编码）
		if strings.Contains(filename, pattern) || strings.Contains(decodedFilename, pattern) {
			zap.S().Infof("文件模糊匹配删除: %s (decoded: %s, pattern=%s)", filename, decodedFilename, pattern)
			return true
		}
		// 支持 * 通配符（尝试两种编码）
		if strings.Contains(pattern, "*") {
			matched, err := filepath.Match(pattern, filename)
			if err == nil && matched {
				zap.S().Infof("文件通配符匹配删除: %s (pattern=%s)", filename, pattern)
				return true
			}
			// 尝试解码后的文件名
			matched, err = filepath.Match(pattern, decodedFilename)
			if err == nil && matched {
				zap.S().Infof("文件通配符匹配删除(decoded): %s (pattern=%s)", decodedFilename, pattern)
				return true
			}
		}
	}
	return false
}

// shouldNotEncrypt 检查文件是否不需要加密（支持模糊匹配）
func shouldNotEncrypt(filename string, noEncryptFiles []string) bool {
	// 如果没有指定不加密列表，所有文件都需要加密
	if len(noEncryptFiles) == 0 {
		return false
	}
	
	// 获取 UTF-8 解码后的文件名（可能是 GBK 编码）
	decodedFilename := decodeGBKFilename(filename)
	
	for _, pattern := range noEncryptFiles {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		// 完全匹配
		if filename == pattern || decodedFilename == pattern {
			zap.S().Debugf("文件不需要加密(完全匹配): %s", filename)
			return true
		}
		// 模糊匹配
		if strings.Contains(filename, pattern) || strings.Contains(decodedFilename, pattern) {
			zap.S().Debugf("文件不需要加密(模糊匹配): %s (pattern=%s)", filename, pattern)
			return true
		}
		// 支持通配符
		if strings.Contains(pattern, "*") {
			matched, err := filepath.Match(pattern, filename)
			if err == nil && matched {
				zap.S().Debugf("文件不需要加密(通配符匹配): %s (pattern=%s)", filename, pattern)
				return true
			}
			matched, err = filepath.Match(pattern, decodedFilename)
			if err == nil && matched {
				zap.S().Debugf("文件不需要加密(通配符匹配 decoded): %s (pattern=%s)", decodedFilename, pattern)
				return true
			}
		}
	}
	return false
}

// applyRenameRules 应用重命名规则
func applyRenameRules(filename string, renameRules []string) string {
	for _, rule := range renameRules {
		parts := strings.SplitN(rule, ":", 2)
		if len(parts) == 2 {
			oldName := strings.TrimSpace(parts[0])
			newName := strings.TrimSpace(parts[1])
			if filename == oldName {
				return newName
			}
			// 支持字符串替换
			if strings.Contains(filename, oldName) {
				return strings.ReplaceAll(filename, oldName, newName)
			}
		}
	}
	return filename
}

// ListZipFiles 列出 ZIP 中的所有文件
func ListZipFiles(data []byte) ([]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var files []string
	for _, file := range reader.File {
		files = append(files, file.Name)
	}

	return files, nil
}

// ExtractFileFromZip 从 ZIP 中提取指定文件
func ExtractFileFromZip(data []byte, filename string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if file.Name == filename {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			return io.ReadAll(rc)
		}
	}

	return nil, os.ErrNotExist
}

// IsZipFile 检查数据是否为有效的 ZIP 文件
func IsZipFile(data []byte) bool {
	// ZIP 文件的魔数是 0x50 0x4B 0x03 0x04
	if len(data) < 4 {
		return false
	}
	return data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}