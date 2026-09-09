package api

import (
	"net/http"

	"nofx/auth"
	"nofx/logger"

	"github.com/gin-gonic/gin"
)

// handleGetUserProfile returns the authenticated user's profile (no secrets).
func (s *Server) handleGetUserProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := s.store.User().GetByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"otp_verified": user.OTPVerified,
		"created_at":   user.CreatedAt,
	})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// handleChangePassword updates the password after verifying the old one.
func (s *Server) handleChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Old password and new password (min 8 chars) are required"})
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Old password is incorrect"})
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	if err := s.store.User().UpdatePassword(userID, newHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}
	logger.Infof("🔑 User %s changed password", MaskEmail(user.Email))
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// handleReset2FA generates a fresh TOTP secret, disables 2FA until re-confirmed,
// and returns the new secret + QR code URL.
func (s *Server) handleReset2FA(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := s.store.User().GetByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	secret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
		return
	}
	if err := s.store.User().UpdateOTPSecret(userID, secret, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update secret"})
		return
	}
	logger.Infof("🔐 User %s reset 2FA secret", MaskEmail(user.Email))
	c.JSON(http.StatusOK, gin.H{
		"secret":      secret,
		"qr_code_url": auth.GetOTPQRCodeURL(secret, user.Email),
	})
}

type confirm2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// handleConfirm2FA verifies a TOTP code against the pending secret and
// re-enables 2FA.
func (s *Server) handleConfirm2FA(c *gin.Context) {
	userID := c.GetString("user_id")
	var req confirm2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code is required"})
		return
	}

	user, err := s.store.User().GetByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !auth.VerifyOTP(user.OTPSecret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}
	if err := s.store.User().UpdateOTPVerified(userID, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2FA"})
		return
	}
	logger.Infof("✅ User %s re-enabled 2FA", MaskEmail(user.Email))
	c.JSON(http.StatusOK, gin.H{"message": "2FA enabled"})
}
