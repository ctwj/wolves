package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"moss/domain/core/entity"
	"moss/domain/core/service"
	"moss/domain/core/vo"
	_ "moss/startup"

	"github.com/xuri/excelize/v2"
)

const (
	baseURL = "https://www.itmop.com"
)

var (
	excelFile string
	batchSize = 100
)

func main() {
	flag.StringVar(&excelFile, "excel", "", "Excel 文件路径")
	flag.IntVar(&batchSize, "batch", 100, "每批处理数量")
	flag.Parse()

	if excelFile == "" {
		log.Fatal("请指定 Excel 文件路径: -excel <path>")
	}

	log.Println("开始导入 Excel 文件:", excelFile)
	log.Println("每批处理数量:", batchSize)

	// 打开 Excel 文件
	f, err := excelize.OpenFile(excelFile)
	if err != nil {
		log.Fatal("打开 Excel 文件失败:", err)
	}
	defer f.Close()

	// 获取所有行
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		log.Fatal("读取 Excel 失败:", err)
	}

	if len(rows) <= 1 {
		log.Fatal("Excel 文件为空或只有标题行")
	}

	// 解析标题行
	headers := rows[0]
	log.Printf("标题行: %v\n", headers)

	// 映射列索引
	colMap := make(map[string]int)
	for i, header := range headers {
		colMap[header] = i
	}

	totalRows := len(rows) - 1
	log.Printf("总记录数: %d\n", totalRows)

	// 批量处理
	var articles []entity.Article
	successCount := 0
	failCount := 0
	startTime := time.Now()

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		article, err := parseRow(row, colMap)
		if err != nil {
			log.Printf("解析第 %d 行失败: %v\n", i, err)
			failCount++
			continue
		}

		articles = append(articles, *article)

		// 达到批量大小或最后一批时创建文章
		if len(articles) >= batchSize || i == totalRows {
			if err := service.Article.CreateInBatches(articles); err != nil {
				log.Printf("批量创建文章失败 [%d-%d]: %v\n", i-len(articles)+1, i, err)
				failCount += len(articles)
			} else {
				log.Printf("批量创建文章成功 [%d-%d] (%d篇)\n", i-len(articles)+1, i, len(articles))
				successCount += len(articles)
			}
			articles = nil // 清空批次数组
		}

		// 进度显示
		if i%10 == 0 || i == totalRows {
			log.Printf("进度: %d/%d (%.1f%%), 成功: %d, 失败: %d\n",
				i, totalRows, float64(i)/float64(totalRows)*100, successCount, failCount)
		}
	}

	duration := time.Since(startTime)
	log.Println("导入完成!")
	log.Printf("总记录数: %d, 成功: %d, 失败: %d\n", totalRows, successCount, failCount)
	log.Printf("耗时: %v\n", duration)
}

// parseRow 解析 Excel 行数据为 entity.Article
func parseRow(row []string, colMap map[string]int) (*entity.Article, error) {
	getCell := func(key string) string {
		if idx, ok := colMap[key]; ok && idx < len(row) {
			return row[idx]
		}
		return ""
	}

	title := getCell("标题")
	if title == "" {
		return nil, fmt.Errorf("标题不能为空")
	}

	content := getCell("完整介绍源码版")
	if content == "" {
		return nil, fmt.Errorf("内容不能为空")
	}

	// 生成 Slug
	slug := generateSlug(title)

	// 构建 Extends
	extends := buildExtends(
		getCell("类型"),
		getCell("操作系统"),
		getCell("语言"),
		getCell("许可证类型"),
		getCell("版本号"),
		getCell("文件大小"),
		getCell("更新时间"),
	)

	// 构建 Res（下载链接）
	res := buildRes(
		getCell("下载地址"),
		getCell("百度网盘"),
		getCell("迅雷网盘"),
		getCell("夸克网盘"),
		getCell("UC网盘"),
		getCell("多网盘"),
	)

	article := &entity.Article{
		ArticleBase: entity.ArticleBase{
			Slug:         slug,
			Title:        title,
			Description:  getCell("描述"),
			CreateTime:   time.Now().Unix(),
			CategoryID:   0,
			Views:        0,
			Thumbnail:    "",
			Status:       false, // 未发布状态
		},
		ArticleDetail: entity.ArticleDetail{
			Keywords: getCell("关键词"),
			Content:  content,
			Extends:  extends,
			Res:      res,
		},
	}

	return article, nil
}

// generateSlug 生成唯一的 Slug
func generateSlug(title string) string {
	hash := md5.Sum([]byte(title))
	return fmt.Sprintf("%x", hash)[:12]
}

// buildExtends 构建 Extends 字段
func buildExtends(fileType, os, language, license, version, fileSize, updateTime string) vo.Extends {
	return vo.Extends{
		{Key: "type", Value: fileType},
		{Key: "os", Value: os},
		{Key: "language", Value: language},
		{Key: "license", Value: license},
		{Key: "version", Value: version},
		{Key: "fileSize", Value: fileSize},
		{Key: "updateTime", Value: updateTime},
	}
}

// buildRes 构建 Res 字段（下载链接数组）
func buildRes(downloadAddr, baiduPan, xunleiPan, quarkPan, ucPan, multiPan string) vo.Extends {
	var downloadLinks []map[string]string

	// 处理下载地址（直链）
	if downloadAddr != "" {
		url := downloadAddr
		if strings.HasPrefix(downloadAddr, "/") {
			url = baseURL + downloadAddr
		}
		// 根据域名判断实际网盘类型
		panType := detectPanType(url)
		downloadLinks = append(downloadLinks, map[string]string{
			"type": panType,
			"url":  url,
		})
	}

	// 处理百度网盘
	if baiduPan != "" {
		downloadLinks = append(downloadLinks, map[string]string{
			"type": "百度网盘",
			"url":  baiduPan,
		})
	}

	// 处理迅雷网盘
	if xunleiPan != "" {
		downloadLinks = append(downloadLinks, map[string]string{
			"type": "迅雷网盘",
			"url":  xunleiPan,
		})
	}

	// 处理夸克网盘
	if quarkPan != "" {
		downloadLinks = append(downloadLinks, map[string]string{
			"type": "夸克网盘",
			"url":  quarkPan,
		})
	}

	// 处理 UC 网盘
	if ucPan != "" {
		downloadLinks = append(downloadLinks, map[string]string{
			"type": "UC网盘",
			"url":  ucPan,
		})
	}

	// 处理多网盘
	if multiPan != "" {
		downloadLinks = append(downloadLinks, map[string]string{
			"type": "多网盘",
			"url":  multiPan,
		})
	}

	// 如果有下载链接，构建 Res 字段
	if len(downloadLinks) > 0 {
		return vo.Extends{
			{Key: "download_links", Value: downloadLinks},
		}
	}

	return vo.Extends{}
}

// detectPanType 根据URL域名判断网盘类型
func detectPanType(url string) string {
	urlLower := strings.ToLower(url)

	// 123云盘
	if strings.Contains(urlLower, "123pan.com") ||
		strings.Contains(urlLower, "123pan.cn") ||
		strings.Contains(urlLower, "123684.com") ||
		strings.Contains(urlLower, "123865.com") ||
		strings.Contains(urlLower, "123685.com") ||
		strings.Contains(urlLower, "123912.com") ||
		strings.Contains(urlLower, "123592.com") {
		return "123云盘"
	}

	// 蓝奏云盘
	if strings.Contains(urlLower, "lanzouj.com") ||
		strings.Contains(urlLower, "lanzoub.com") ||
		strings.Contains(urlLower, "lanzou.com") ||
		strings.Contains(urlLower, "lanzoui.com") ||
		strings.Contains(urlLower, "lanzoux.com") {
		return "蓝奏云盘"
	}

	// 天翼云盘
	if strings.Contains(urlLower, "cloud.189.cn") {
		return "天翼云盘"
	}

	// 迅雷云盘
	if strings.Contains(urlLower, "pan.xunlei.com") {
		return "迅雷云盘"
	}

	// 115云盘
	if strings.Contains(urlLower, "115.com") ||
		strings.Contains(urlLower, "115cdn.com") ||
		strings.Contains(urlLower, "anxia.com") {
		return "115云盘"
	}

	// UC云盘
	if strings.Contains(urlLower, "drive.uc.cn") {
		return "UC云盘"
	}

	// 阿里云盘
	if strings.Contains(urlLower, "aliyundrive.com") ||
		strings.Contains(urlLower, "alipan.com") {
		return "阿里云盘"
	}

	// 百度网盘
	if strings.Contains(urlLower, "pan.baidu.com") {
		return "百度网盘"
	}

	// 夸克网盘
	if strings.Contains(urlLower, "pan.quark.cn") {
		return "夸克网盘"
	}

	// 默认为直链
	return "直链"
}