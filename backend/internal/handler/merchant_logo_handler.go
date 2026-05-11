// MERCHANT-SYSTEM v1.0
// 商户 logo 上传 + 静态服务。
//
// 上传：POST /api/v1/merchant/upload/logo （JWT，必须是 merchant owner）
//   - multipart/form-data，字段名 file
//   - 限 image/png|jpeg|webp，单文件 ≤ 1MB
//   - 不收 SVG：SVG 可含 <script>，主域名 /merchant-assets/.../evil.svg 直接
//     访问会形成 stored XSS（PNG/JPEG/WebP 走 <img> 渲染管线，浏览器不会执行脚本）
//   - 存盘 data/merchant-uploads/{merchant_id}/logo-{unix_nano}.{ext}
//   - 返回 { url: "/merchant-assets/{merchant_id}/{filename}" }
//   - 不自动改 merchant_domains.site_logo，前端拿到 url 再走原有的 UpdateDomain 接口
//
// 静态：GET /merchant-assets/:merchant_id/:filename
//   - 公开（logos 本来就是公开品牌资产）
//   - 拒绝路径穿越；只服务 data/merchant-uploads/{merchant_id}/ 下的文件

package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	merchantUploadDir      = "data/merchant-uploads"
	merchantLogoMaxBytes   = 1 << 20 // 1MB
	merchantLogoMultipart  = 2 << 20 // 2MB（multipart 解析上限，比文件上限稍大）
	merchantLogoFormField  = "file"
	merchantAssetURLPrefix = "/merchant-assets"
)

// merchantLogoAllowed MIME → 文件扩展名（不含 SVG，详见文件头注释）。
var merchantLogoAllowed = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// MerchantLogoHandler logo 上传 / 静态服务。
type MerchantLogoHandler struct {
	merchantSvc *service.MerchantService
}

func NewMerchantLogoHandler(merchantSvc *service.MerchantService) *MerchantLogoHandler {
	return &MerchantLogoHandler{merchantSvc: merchantSvc}
}

// Upload 接收当前 JWT 用户（必须是 merchant owner）的 logo 上传。
// 成功返回 { url: "/merchant-assets/.../..." }，前端把这个 url 写到 merchant_domains.site_logo。
func (h *MerchantLogoHandler) Upload(c *gin.Context) {
	userID, ok := jwtUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	m, err := h.merchantSvc.GetByOwnerUserID(c.Request.Context(), userID)
	if err != nil || m == nil {
		response.Forbidden(c, "not a merchant owner")
		return
	}

	// 控制 multipart 解析上限（防大文件耗内存）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, merchantLogoMultipart)
	if err := c.Request.ParseMultipartForm(merchantLogoMultipart); err != nil {
		response.BadRequest(c, "upload too large or invalid multipart")
		return
	}

	file, header, err := c.Request.FormFile(merchantLogoFormField)
	if err != nil {
		response.BadRequest(c, "missing file field")
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size > merchantLogoMaxBytes {
		response.BadRequest(c, "file too large (max 1MB)")
		return
	}

	// 取 Content-Type 自报；要更严的话再 sniff 文件头（这里依赖前端 + 扩展名）
	ct := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	ext, ok := merchantLogoAllowed[ct]
	if !ok {
		response.BadRequest(c, "unsupported image type; allow png/jpeg/webp")
		return
	}

	// 落盘 data/merchant-uploads/{merchant_id}/logo-{nano}.{ext}
	dir := filepath.Join(merchantUploadDir, strconv.FormatInt(m.ID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		response.InternalError(c, "create upload dir: "+err.Error())
		return
	}
	filename := fmt.Sprintf("logo-%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(dir, filename)

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		response.InternalError(c, "write file: "+err.Error())
		return
	}
	written, copyErr := io.Copy(out, file)
	_ = out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		response.InternalError(c, "write file: "+copyErr.Error())
		return
	}
	if written > merchantLogoMaxBytes {
		_ = os.Remove(dst)
		response.BadRequest(c, "file too large (max 1MB)")
		return
	}

	url := fmt.Sprintf("%s/%d/%s", merchantAssetURLPrefix, m.ID, filename)
	response.Success(c, gin.H{"url": url})
}

// Serve 公开提供商户 logo 静态文件。
// 路径形如 /merchant-assets/:merchant_id/:filename
// 拒绝路径穿越（filename 含 / .. 等一律 400）。
func (h *MerchantLogoHandler) Serve(c *gin.Context) {
	merchantIDStr := c.Param("merchant_id")
	filename := c.Param("filename")

	if _, err := strconv.ParseInt(merchantIDStr, 10, 64); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if filename == "" || strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
		c.Status(http.StatusBadRequest)
		return
	}

	path := filepath.Join(merchantUploadDir, merchantIDStr, filename)

	// 再做一次最终路径检查（确保 join 后没逃出 merchantUploadDir）
	absBase, _ := filepath.Abs(merchantUploadDir)
	absPath, _ := filepath.Abs(path)
	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) {
		c.Status(http.StatusBadRequest)
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.File(path)
}
