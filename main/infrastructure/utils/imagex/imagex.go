package imagex

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"math"
	_ "image/png"

	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
	"github.com/nfnt/resize"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
	"moss/resources"
)

// WatermarkType 水印类型
type WatermarkType string

const (
	WatermarkTypeNone  WatermarkType = "none"
	WatermarkTypeText  WatermarkType = "text"
	WatermarkTypeImage WatermarkType = "image"
)

// WatermarkPosition 水印位置
type WatermarkPosition string

const (
	PositionTopLeft     WatermarkPosition = "top_left"
	PositionTopRight    WatermarkPosition = "top_right"
	PositionBottomLeft  WatermarkPosition = "bottom_left"
	PositionBottomRight WatermarkPosition = "bottom_right"
	PositionCenter      WatermarkPosition = "center"
	PositionTile        WatermarkPosition = "tile"
)

// WatermarkConfig 水印基础配置
type WatermarkConfig struct {
	Enabled     bool              // 是否启用水印
	Type        WatermarkType     // 水印类型: text/image
	Position    WatermarkPosition // 水印位置
	Opacity     float64           // 透明度 (0-1), 1为不透明
	Margin      int               // 边距(像素),用于非中心位置
	TileSpacing int               // 平铺间距(像素),仅在平铺模式有效
	MinWidth    int               // 最小宽度限制(像素),只有图片宽度大于等于此值才添加水印
}

// TextWatermarkConfig 文字水印配置
type TextWatermarkConfig struct {
	WatermarkConfig
	Text        string // 水印文字
	FontSize    int    // 字体大小(像素)
	FontColor   string // 字体颜色 (十六进制,如 "#FF0000")
	RotateAngle int    // 旋转角度(度),0表示不旋转
	BgColor     string // 背景颜色 (十六进制,如 "#000000"), 空字符串表示无背景
	BgRadius    int    // 背景圆角半径(像素),0表示无圆角
	// 新增：描边配置
	StrokeColor string // 描边颜色 (十六进制,如 "#000000"), 空字符串表示无描边
	StrokeWidth int    // 描边宽度(像素)
	// 新增：渐变背景配置
	BgGradientStart string // 渐变起始颜色 (十六进制), 与 BgColor 互斥
	BgGradientEnd   string // 渐变结束颜色 (十六进制)
	BgGradientAngle int    // 渐变角度 (0-360), 0=从左到右, 90=从上到下
	BgPadding       int    // 背景内边距(像素), 默认为字体大小的1/3
}

// ImageWatermarkConfig 图片水印配置
type ImageWatermarkConfig struct {
	WatermarkConfig
	ImageData   []byte  // 水印图片数据
	ScaleRatio  float64 // 缩放比例(0-1),相对于原图的比例
	RotateAngle int     // 旋转角度(度)
}

// Image 图片处理工具
// 包括缩放、提取缩略图、计算宽高比、水印添加
type Image struct {
	width  int
	height int

	// 图片缩放算法
	// NearestNeighbor: Nearest-neighbor插值
	// Bilinear：双线性插值
	// Bicubic：双三次插值
	// MitchellNetravali:Mitchell-Netravali插值
	// Lanczos2:Lanczos重采样，a=2
	// Lanczos3:Lanczos重采样，a=3
	interp resize.InterpolationFunction

	// 水印配置
	watermarkConfig *WatermarkConfig
	textWatermark   *TextWatermarkConfig
	imageWatermark  *ImageWatermarkConfig
}

func New() *Image {
	return &Image{}
}

func (i *Image) SetWidth(n int) *Image {
	i.width = n
	return i
}

func (i *Image) SetHeight(n int) *Image {
	i.height = n
	return i
}

func (i *Image) SetInterp(interp resize.InterpolationFunction) *Image {
	i.interp = interp
	return i
}

// Resize 缩放图片
// 宽或高有一项为0，则等比例缩放
func (i *Image) Resize(img image.Image) image.Image {
	return resize.Resize(uint(i.width), uint(i.height), img, i.interp)
}

// ResizeByte 缩放图片 by []byte
func (i *Image) ResizeByte(b []byte) ([]byte, error) {
	img, err := i.Byte2Image(b)
	if err != nil {
		return []byte{}, err
	}
	img = i.Resize(img)
	return i.Image2Byte(img), nil
}

// Thumbnail 生成缩略图
// 如果图片小于设置的尺寸，则返回原图片
func (i *Image) Thumbnail(img image.Image) image.Image {
	return resize.Thumbnail(uint(i.width), uint(i.height), img, i.interp)
}

func (i *Image) ThumbnailByte(b []byte) (_ []byte, err error) {
	img, err := i.Byte2Image(b)
	if err != nil {
		return
	}
	imgN := resize.Thumbnail(uint(i.width), uint(i.height), img, i.interp)
	return i.Image2Byte(imgN), nil
}

// Byte2Image []byte转image
func (i *Image) Byte2Image(b []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// Image2Byte image转[]byte
func (i *Image) Image2Byte(img image.Image) []byte {
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

// CropByte 智能提取图片 by byte
// 按照尺寸，智能提取图片核心部分
func (i *Image) CropByte(b []byte) ([]byte, error) {
	img, err := i.Byte2Image(b)
	if err != nil {
		return []byte{}, err
	}
	if img, err = i.Crop(img); err != nil {
		return []byte{}, err
	}
	return i.Image2Byte(img), nil
}

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// Crop 智能提取图片
// 按照尺寸，智能提取图片核心部分
func (i *Image) Crop(img image.Image) (image.Image, error) {
	analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
	topCrop, err := analyzer.FindBestCrop(img, i.width, i.height)
	if err != nil {
		return nil, err
	}
	croppedImg := img.(subImager).SubImage(topCrop)
	if croppedImg.Bounds().Dx() > i.width { // 当前提取的图片大于设定的尺寸
		croppedImg = i.Resize(croppedImg) // 缩放
	}
	return croppedImg, nil
}

// ComputeScale 通过最大宽高 按比例计算新的宽高
// 如果未超过最大宽高，则原样返回
// 最大宽高设置0则不限制
func ComputeScale(width, height, maxWidth, maxHeight int) (int, int) {
	if maxWidth > 0 && width > maxWidth {
		var scale = float64(width) / float64(height)
		width = maxWidth
		height = int(float64(width) / scale)
	}
	if maxHeight > 0 && height > maxHeight {
		var scale = float64(height) / float64(width)
		height = maxHeight
		width = int(float64(height) / scale)
	}
	return width, height
}

// drawRoundedRect 绘制圆角矩形
func drawRoundedRect(img *image.RGBA, x, y, width, height, radius int, color color.RGBA) {
	if radius < 0 {
		radius = 0
	}
	if radius > width/2 {
		radius = width / 2
	}
	if radius > height/2 {
		radius = height / 2
	}

	// 绘制四个角
	for cy := 0; cy < radius; cy++ {
		for cx := 0; cx < radius; cx++ {
			// 检查点是否在圆内
			dx := radius - cx
			dy := radius - cy
			if dx*dx+dy*dy <= radius*radius {
				// 左上角
				img.Set(x+cx, y+cy, color)
				// 右上角
				img.Set(x+width-cx-1, y+cy, color)
				// 左下角
				img.Set(x+cx, y+height-cy-1, color)
				// 右下角
				img.Set(x+width-cx-1, y+height-cy-1, color)
			}
		}
	}

	// 绘制中间区域（不包括圆角部分）
	for row := radius; row < height-radius; row++ {
		for col := 0; col < width; col++ {
			img.Set(x+col, y+row, color)
		}
	}

	// 绘制左右边缘
	for row := 0; row < radius; row++ {
		for col := radius; col < width-radius; col++ {
			img.Set(x+col, y+row, color)
			img.Set(x+col, y+height-row-1, color)
		}
	}
}

// SetWatermarkConfig 设置水印基础配置
func (i *Image) SetWatermarkConfig(config *WatermarkConfig) *Image {
	i.watermarkConfig = config
	return i
}

// SetTextWatermark 设置文字水印参数
func (i *Image) SetTextWatermark(text string, fontSize int, fontColor string, rotateAngle int, bgColor string, bgRadius int) *Image {
	i.textWatermark = &TextWatermarkConfig{
		Text:        text,
		FontSize:    fontSize,
		FontColor:   fontColor,
		RotateAngle: rotateAngle,
		BgColor:     bgColor,
		BgRadius:    bgRadius,
	}
	return i
}

// SetTextWatermarkEx 设置文字水印参数（扩展版，支持描边和渐变）
func (i *Image) SetTextWatermarkEx(text string, fontSize int, fontColor string, rotateAngle int, bgColor string, bgRadius int, strokeColor string, strokeWidth int, bgGradientStart string, bgGradientEnd string, bgGradientAngle int, bgPadding int) *Image {
	i.textWatermark = &TextWatermarkConfig{
		Text:            text,
		FontSize:        fontSize,
		FontColor:       fontColor,
		RotateAngle:     rotateAngle,
		BgColor:         bgColor,
		BgRadius:        bgRadius,
		StrokeColor:     strokeColor,
		StrokeWidth:     strokeWidth,
		BgGradientStart: bgGradientStart,
		BgGradientEnd:   bgGradientEnd,
		BgGradientAngle: bgGradientAngle,
		BgPadding:       bgPadding,
	}
	return i
}

// SetImageWatermark 设置图片水印参数
func (i *Image) SetImageWatermark(imageData []byte, scaleRatio float64, rotateAngle int) *Image {
	i.imageWatermark = &ImageWatermarkConfig{
		ImageData:   imageData,
		ScaleRatio:  scaleRatio,
		RotateAngle: rotateAngle,
	}
	return i
}

// AddWatermark 根据配置添加水印(统一入口)
func (i *Image) AddWatermark(img image.Image) (image.Image, error) {
	if i.watermarkConfig == nil || !i.watermarkConfig.Enabled {
		return img, nil
	}

	// 检查图片宽度是否满足最小宽度要求
	if i.watermarkConfig.MinWidth > 0 {
		bounds := img.Bounds()
		imgWidth := bounds.Dx()
		if imgWidth < i.watermarkConfig.MinWidth {
			return img, nil
		}
	}

	switch i.watermarkConfig.Type {
	case WatermarkTypeText:
		if i.textWatermark != nil {
			return i.AddTextWatermark(img)
		}
	case WatermarkTypeImage:
		if i.imageWatermark != nil {
			return i.AddImageWatermark(img)
		}
	}

	return img, nil
}

// AddWatermarkByte 对字节数据添加水印
func (i *Image) AddWatermarkByte(b []byte) ([]byte, error) {
	img, err := i.Byte2Image(b)
	if err != nil {
		return []byte{}, err
	}

	img, err = i.AddWatermark(img)
	if err != nil {
		return []byte{}, err
	}

	return i.Image2Byte(img), nil
}

// AddTextWatermark 添加文字水印
func (i *Image) AddTextWatermark(img image.Image) (image.Image, error) {
	if i.textWatermark == nil || i.textWatermark.Text == "" {
		return img, nil
	}

	config := i.watermarkConfig
	textConfig := i.textWatermark

	// 创建一个新的 RGBA 图片用于绘制
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// 解析颜色
	fontColor, err := parseHexColor(textConfig.FontColor)
	if err != nil {
		fontColor = color.RGBA{255, 255, 255, 255} // 默认白色
	}

	// 解析描边颜色
	var strokeColor color.RGBA
	var hasStroke bool
	if textConfig.StrokeColor != "" && textConfig.StrokeWidth > 0 {
		strokeColor, err = parseHexColor(textConfig.StrokeColor)
		hasStroke = err == nil
	}

	// 加载字体文件（优先使用中文字体，支持中文水印）
	fontBytes, err := resources.App.ReadFile("app/font.ttf") // 优先加载自定义中文字体
	if err != nil {
		fontBytes, err = resources.App.ReadFile("app/comic.ttf") // 回退到默认英文字体
	}
	if err != nil {
		// 回退到基础字体
		return i.drawTextWithBasicFont(rgba, textConfig, config, fontColor, strokeColor, hasStroke, basicfont.Face7x13)
	}

	// 解析字体
	opFont, err := opentype.Parse(fontBytes)
	if err != nil {
		return i.drawTextWithBasicFont(rgba, textConfig, config, fontColor, strokeColor, hasStroke, basicfont.Face7x13)
	}

	// 创建可缩放字体
	fontSize := textConfig.FontSize
	if fontSize < 12 {
		fontSize = 12
	}

	face, err := opentype.NewFace(opFont, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return i.drawTextWithBasicFont(rgba, textConfig, config, fontColor, strokeColor, hasStroke, basicfont.Face7x13)
	}

	// 计算文字尺寸
	textWidth := font.MeasureString(face, textConfig.Text).Ceil()
	textHeight := face.Metrics().Height.Ceil()
	ascent := face.Metrics().Ascent.Ceil()
	descent := face.Metrics().Descent.Ceil()

	// 计算背景内边距（优先使用自定义配置）
	padding := textConfig.BgPadding
	if padding <= 0 {
		padding = fontSize / 3
	}

	// 计算水印图层尺寸
	bgWidth := textWidth + padding*2
	bgHeight := textHeight + padding*2

	// 创建独立的水印图层
	watermarkLayer := image.NewRGBA(image.Rect(0, 0, bgWidth, bgHeight))

	// 在水印图层上绘制背景
	i.drawBackgroundOnLayer(watermarkLayer, textConfig, config, 0, 0, bgWidth, bgHeight)

	// 计算文字绘制位置（真正垂直居中）
	textX := padding
	textY := padding + ascent + (textHeight-ascent-descent)/2

	// 绘制描边（如果启用）
	if hasStroke {
		strokeCol := color.NRGBA{
			R: uint8(strokeColor.R),
			G: uint8(strokeColor.G),
			B: uint8(strokeColor.B),
			A: uint8(config.Opacity * 255),
		}
		i.drawTextWithStroke(watermarkLayer, face, textConfig.Text, textX, textY, strokeCol, textConfig.StrokeWidth)
	}

	// 绘制文字
	fgCol := color.NRGBA{
		R: uint8(fontColor.R),
		G: uint8(fontColor.G),
		B: uint8(fontColor.B),
		A: uint8(config.Opacity * 255),
	}

	drawer := font.Drawer{
		Dst:  watermarkLayer,
		Src:  image.NewUniform(fgCol),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(textX), Y: fixed.I(textY)},
	}
	drawer.DrawString(textConfig.Text)

	// 应用旋转（如果有）
	if textConfig.RotateAngle != 0 {
		watermarkLayer = rotateImage(watermarkLayer, float64(textConfig.RotateAngle)).(*image.RGBA)
	}

	// 计算水印位置并绘制到原图
	wmBounds := watermarkLayer.Bounds()
	x, y := i.computeWatermarkPosition(bounds.Dx(), bounds.Dy(), wmBounds.Dx(), wmBounds.Dy())
	i.applyWatermarkWithOpacity(rgba, watermarkLayer, x, y, 1.0) // 透明度已在绘制时应用

	return rgba, nil
}

// drawTextWithBasicFont 使用基础字体绘制文字
func (i *Image) drawTextWithBasicFont(rgba *image.RGBA, textConfig *TextWatermarkConfig, config *WatermarkConfig, fontColor, strokeColor color.RGBA, hasStroke bool, face font.Face) (image.Image, error) {
	bounds := rgba.Bounds()
	fontSize := textConfig.FontSize
	if fontSize < 12 {
		fontSize = 12
	}

	// 计算文字尺寸
	textWidth := font.MeasureString(face, textConfig.Text).Ceil()
	textHeight := face.Metrics().Height.Ceil()
	ascent := face.Metrics().Ascent.Ceil()
	descent := face.Metrics().Descent.Ceil()

	// 计算背景内边距
	padding := textConfig.BgPadding
	if padding <= 0 {
		padding = fontSize / 3
	}

	// 计算水印图层尺寸
	bgWidth := textWidth + padding*2
	bgHeight := textHeight + padding*2

	// 创建独立的水印图层
	watermarkLayer := image.NewRGBA(image.Rect(0, 0, bgWidth, bgHeight))

	// 在水印图层上绘制背景
	i.drawBackgroundOnLayer(watermarkLayer, textConfig, config, 0, 0, bgWidth, bgHeight)

	// 计算文字绘制位置（真正垂直居中）
	textX := padding
	textY := padding + ascent + (textHeight-ascent-descent)/2

	// 绘制描边
	if hasStroke {
		strokeCol := color.NRGBA{
			R: uint8(strokeColor.R),
			G: uint8(strokeColor.G),
			B: uint8(strokeColor.B),
			A: uint8(config.Opacity * 255),
		}
		i.drawTextWithStroke(watermarkLayer, face, textConfig.Text, textX, textY, strokeCol, textConfig.StrokeWidth)
	}

	// 绘制文字
	fgCol := color.NRGBA{
		R: uint8(fontColor.R),
		G: uint8(fontColor.G),
		B: uint8(fontColor.B),
		A: uint8(config.Opacity * 255),
	}

	drawer := font.Drawer{
		Dst:  watermarkLayer,
		Src:  image.NewUniform(fgCol),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(textX), Y: fixed.I(textY)},
	}
	drawer.DrawString(textConfig.Text)

	// 应用旋转（如果有）
	if textConfig.RotateAngle != 0 {
		watermarkLayer = rotateImage(watermarkLayer, float64(textConfig.RotateAngle)).(*image.RGBA)
	}

	// 计算水印位置并绘制到原图
	wmBounds := watermarkLayer.Bounds()
	x, y := i.computeWatermarkPosition(bounds.Dx(), bounds.Dy(), wmBounds.Dx(), wmBounds.Dy())
	i.applyWatermarkWithOpacity(rgba, watermarkLayer, x, y, 1.0)

	return rgba, nil
}

// drawBackground 绘制背景（支持纯色和渐变）
func (i *Image) drawBackground(rgba *image.RGBA, textConfig *TextWatermarkConfig, config *WatermarkConfig, x, y, width, height int) {
	// 检查是否使用渐变背景
	if textConfig.BgGradientStart != "" && textConfig.BgGradientEnd != "" {
		startColor, err1 := parseHexColor(textConfig.BgGradientStart)
		endColor, err2 := parseHexColor(textConfig.BgGradientEnd)
		if err1 == nil && err2 == nil {
			i.drawGradientBackground(rgba, x, y, width, height, textConfig.BgRadius, startColor, endColor, textConfig.BgGradientAngle, config.Opacity)
			return
		}
	}

	// 使用纯色背景
	if textConfig.BgColor != "" {
		bgColor, err := parseHexColor(textConfig.BgColor)
		if err == nil {
			bgColorWithAlpha := color.RGBA{
				R: bgColor.R,
				G: bgColor.G,
				B: bgColor.B,
				A: uint8(config.Opacity * 255),
			}
			drawRoundedRect(rgba, x, y, width, height, textConfig.BgRadius, bgColorWithAlpha)
		}
	}
}

// drawBackgroundOnLayer 在独立图层上绘制背景（从原点开始）
func (i *Image) drawBackgroundOnLayer(rgba *image.RGBA, textConfig *TextWatermarkConfig, config *WatermarkConfig, x, y, width, height int) {
	// 检查是否使用渐变背景
	if textConfig.BgGradientStart != "" && textConfig.BgGradientEnd != "" {
		startColor, err1 := parseHexColor(textConfig.BgGradientStart)
		endColor, err2 := parseHexColor(textConfig.BgGradientEnd)
		if err1 == nil && err2 == nil {
			i.drawGradientBackground(rgba, x, y, width, height, textConfig.BgRadius, startColor, endColor, textConfig.BgGradientAngle, 1.0)
			return
		}
	}

	// 使用纯色背景
	if textConfig.BgColor != "" {
		bgColor, err := parseHexColor(textConfig.BgColor)
		if err == nil {
			bgColorWithAlpha := color.RGBA{
				R: bgColor.R,
				G: bgColor.G,
				B: bgColor.B,
				A: uint8(config.Opacity * 255),
			}
			drawRoundedRect(rgba, x, y, width, height, textConfig.BgRadius, bgColorWithAlpha)
		}
	}
}

// drawGradientBackground 绘制渐变背景
func (i *Image) drawGradientBackground(rgba *image.RGBA, x, y, width, height, radius int, startColor, endColor color.RGBA, angle int, opacity float64) {
	// 确保圆角半径有效
	if radius < 0 {
		radius = 0
	}
	if radius > width/2 {
		radius = width / 2
	}
	if radius > height/2 {
		radius = height / 2
	}

	// 计算渐变方向
	// angle: 0=从左到右, 90=从上到下, 180=从右到左, 270=从下到上
	angle = angle % 360
	if angle < 0 {
		angle += 360
	}

	// 遍历每个像素绘制渐变
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// 检查是否在圆角矩形内
			px, py := x+col, y+row
			if !isInRoundedRect(col, row, width, height, radius) {
				continue
			}

			// 计算渐变比例
			var ratio float64
			switch {
			case angle >= 0 && angle < 45 || angle >= 315 && angle < 360:
				// 从左到右
				ratio = float64(col) / float64(width)
			case angle >= 45 && angle < 135:
				// 从上到下
				ratio = float64(row) / float64(height)
			case angle >= 135 && angle < 225:
				// 从右到左
				ratio = 1.0 - float64(col)/float64(width)
			case angle >= 225 && angle < 315:
				// 从下到上
				ratio = 1.0 - float64(row)/float64(height)
			}

			// 插值计算颜色
			r := uint8(float64(startColor.R)*(1-ratio) + float64(endColor.R)*ratio)
			g := uint8(float64(startColor.G)*(1-ratio) + float64(endColor.G)*ratio)
			b := uint8(float64(startColor.B)*(1-ratio) + float64(endColor.B)*ratio)
			a := uint8(opacity * 255)

			rgba.SetRGBA(px, py, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
}

// isInRoundedRect 检查点是否在圆角矩形内
func isInRoundedRect(col, row, width, height, radius int) bool {
	// 检查四个角落
	// 左上角
	if col < radius && row < radius {
		dx := radius - col
		dy := radius - row
		return dx*dx+dy*dy <= radius*radius
	}
	// 右上角
	if col >= width-radius && row < radius {
		dx := col - (width - radius - 1)
		dy := radius - row
		return dx*dx+dy*dy <= radius*radius
	}
	// 左下角
	if col < radius && row >= height-radius {
		dx := radius - col
		dy := row - (height - radius - 1)
		return dx*dx+dy*dy <= radius*radius
	}
	// 右下角
	if col >= width-radius && row >= height-radius {
		dx := col - (width - radius - 1)
		dy := row - (height - radius - 1)
		return dx*dx+dy*dy <= radius*radius
	}
	// 中间区域
	return true
}

// drawTextWithStroke 绘制带描边的文字
func (i *Image) drawTextWithStroke(rgba *image.RGBA, face font.Face, text string, x, y int, strokeColor color.NRGBA, strokeWidth int) {
	// 在文字周围绘制多层来模拟描边效果
	offsets := []struct{ dx, dy int }{
		{-strokeWidth, 0}, {strokeWidth, 0}, {0, -strokeWidth}, {0, strokeWidth},
		{-strokeWidth, -strokeWidth}, {strokeWidth, -strokeWidth},
		{-strokeWidth, strokeWidth}, {strokeWidth, strokeWidth},
	}

	for _, offset := range offsets {
		drawer := font.Drawer{
			Dst:  rgba,
			Src:  image.NewUniform(strokeColor),
			Face: face,
			Dot:  fixed.Point26_6{X: fixed.I(x + offset.dx), Y: fixed.I(y + offset.dy)},
		}
		drawer.DrawString(text)
	}
}

// AddImageWatermark 添加图片水印
func (i *Image) AddImageWatermark(img image.Image) (image.Image, error) {
	if i.imageWatermark == nil || len(i.imageWatermark.ImageData) == 0 {
		return img, nil
	}

	config := i.watermarkConfig
	imgConfig := i.imageWatermark

	// 解码水印图片
	watermarkImg, _, err := image.Decode(bytes.NewReader(imgConfig.ImageData))
	if err != nil {
		return nil, err
	}

	// 计算水印图片尺寸
	wmBounds := watermarkImg.Bounds()
	wmWidth := wmBounds.Dx()
	wmHeight := wmBounds.Dy()

	// 根据比例缩放水印图片
	if imgConfig.ScaleRatio > 0 && imgConfig.ScaleRatio < 1 {
		wmWidth = int(float64(wmWidth) * imgConfig.ScaleRatio)
		wmHeight = int(float64(wmHeight) * imgConfig.ScaleRatio)
		watermarkImg = resize.Resize(uint(wmWidth), uint(wmHeight), watermarkImg, resize.Lanczos3)
	}

	// 应用旋转（如果有）
	if imgConfig.RotateAngle != 0 {
		watermarkImg = rotateImage(watermarkImg, float64(imgConfig.RotateAngle))
	}

	// 更新旋转后的尺寸
	wmBounds = watermarkImg.Bounds()
	wmWidth = wmBounds.Dx()
	wmHeight = wmBounds.Dy()

	// 创建目标图片
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// 平铺模式
	if config.Position == PositionTile {
		return i.drawTileWatermark(rgba, watermarkImg), nil
	}

	// 计算水印位置
	x, y := i.computeWatermarkPosition(bounds.Dx(), bounds.Dy(), wmWidth, wmHeight)

	// 应用透明度并绘制水印
	i.applyWatermarkWithOpacity(rgba, watermarkImg, x, y, config.Opacity)

	return rgba, nil
}

// computeWatermarkPosition 计算水印位置
func (i *Image) computeWatermarkPosition(imgWidth, imgHeight, wmWidth, wmHeight int) (x, y int) {
	if i.watermarkConfig == nil {
		return 0, 0
	}

	margin := i.watermarkConfig.Margin
	position := i.watermarkConfig.Position

	switch position {
	case PositionTopLeft:
		x = margin
		y = margin
	case PositionTopRight:
		x = imgWidth - wmWidth - margin
		y = margin
	case PositionBottomLeft:
		x = margin
		y = imgHeight - wmHeight - margin
	case PositionBottomRight:
		x = imgWidth - wmWidth - margin
		y = imgHeight - wmHeight - margin
	case PositionCenter:
		x = (imgWidth - wmWidth) / 2
		y = (imgHeight - wmHeight) / 2
	default:
		// 默认右下角
		x = imgWidth - wmWidth - margin
		y = imgHeight - wmHeight - margin
	}

	return x, y
}

// applyWatermarkWithOpacity 应用透明度并绘制水印
func (i *Image) applyWatermarkWithOpacity(dst *image.RGBA, src image.Image, x, y int, opacity float64) {
	srcBounds := src.Bounds()
	dstBounds := dst.Bounds()

	// 确保水印不超出目标图片范围
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+srcBounds.Dx() > dstBounds.Dx() {
		x = dstBounds.Dx() - srcBounds.Dx()
	}
	if y+srcBounds.Dy() > dstBounds.Dy() {
		y = dstBounds.Dy() - srcBounds.Dy()
	}

	// 遍历水印图片的每个像素
	for sy := srcBounds.Min.Y; sy < srcBounds.Max.Y; sy++ {
		for sx := srcBounds.Min.X; sx < srcBounds.Max.X; sx++ {
			dx := x + (sx - srcBounds.Min.X)
			dy := y + (sy - srcBounds.Min.Y)

			if dx >= 0 && dx < dstBounds.Dx() && dy >= 0 && dy < dstBounds.Dy() {
				// 获取水印像素
				srcColor := color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
				if srcColor.A == 0 {
					continue // 完全透明像素跳过
				}

				// 应用透明度
				srcColor.A = uint8(float64(srcColor.A) * opacity)

				// 获取目标像素
				dstColor := dst.RGBAAt(dx, dy)

				// Alpha 混合
				alpha := float64(srcColor.A) / 255.0
				dstColor.R = uint8(float64(dstColor.R)*(1-alpha) + float64(srcColor.R)*alpha)
				dstColor.G = uint8(float64(dstColor.G)*(1-alpha) + float64(srcColor.G)*alpha)
				dstColor.B = uint8(float64(dstColor.B)*(1-alpha) + float64(srcColor.B)*alpha)
				dstColor.A = 255 // 目标图片不透明

				dst.SetRGBA(dx, dy, dstColor)
			}
		}
	}
}

// drawTileWatermark 绘制平铺水印
func (i *Image) drawTileWatermark(dst *image.RGBA, watermark image.Image) image.Image {
	if i.watermarkConfig == nil {
		return dst
	}

	spacing := i.watermarkConfig.TileSpacing
	if spacing <= 0 {
		spacing = 100
	}

	wmBounds := watermark.Bounds()
	wmWidth := wmBounds.Dx()
	wmHeight := wmBounds.Dy()

	dstBounds := dst.Bounds()
	dstWidth := dstBounds.Dx()
	dstHeight := dstBounds.Dy()

	// 平铺绘制水印
	for y := 0; y < dstHeight; y += wmHeight + spacing {
		for x := 0; x < dstWidth; x += wmWidth + spacing {
			i.applyWatermarkWithOpacity(dst, watermark, x, y, i.watermarkConfig.Opacity)
		}
	}

	return dst
}

// parseHexColor 解析十六进制颜色
func parseHexColor(hex string) (color.RGBA, error) {
	c := color.RGBA{}
	var err error

	switch len(hex) {
	case 7:
		_, err = fmt.Sscanf(hex, "#%02x%02x%02x", &c.R, &c.G, &c.B)
		c.A = 255
	case 4:
		_, err = fmt.Sscanf(hex, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		c.R *= 17
		c.G *= 17
		c.B *= 17
		c.A = 255
	default:
		err = fmt.Errorf("invalid color format: %s", hex)
	}

	return c, err
}

// rotateImage 旋转图片（顺时针方向，角度为度）
func rotateImage(img image.Image, angle float64) image.Image {
	if angle == 0 {
		return img
	}

	// 将角度转换为弧度（顺时针旋转）
	radians := -angle * math.Pi / 180.0

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// 计算旋转后的图片尺寸
	cos := math.Cos(radians)
	sin := math.Sin(radians)
	newW := int(math.Ceil(math.Abs(float64(srcW)*cos) + math.Abs(float64(srcH)*sin)))
	newH := int(math.Ceil(math.Abs(float64(srcW)*sin) + math.Abs(float64(srcH)*cos)))

	// 创建新图片
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	// 计算中心点
	srcCX := float64(srcW) / 2.0
	srcCY := float64(srcH) / 2.0
	dstCX := float64(newW) / 2.0
	dstCY := float64(newH) / 2.0

	// 遍历目标图片的每个像素
	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			// 计算源图片中的对应位置（逆变换）
			px := float64(dx) - dstCX
			py := float64(dy) - dstCY

			// 逆旋转
			sx := px*cos - py*sin + srcCX
			sy := px*sin + py*cos + srcCY

			// 双线性插值
			if sx >= 0 && sx < float64(srcW) && sy >= 0 && sy < float64(srcH) {
				// 简单的最近邻插值
				sxInt := int(sx + 0.5)
				syInt := int(sy + 0.5)
				if sxInt >= 0 && sxInt < srcW && syInt >= 0 && syInt < srcH {
					dst.Set(dx, dy, img.At(sxInt+bounds.Min.X, syInt+bounds.Min.Y))
				}
			}
		}
	}

	return dst
}
