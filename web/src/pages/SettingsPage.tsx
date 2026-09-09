import { useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { t, type Language } from '../i18n/translations'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { toast } from 'sonner'
import {
  Settings as SettingsIcon,
  KeyRound,
  ShieldCheck,
  Loader2,
  Mail,
  BadgeCheck,
} from 'lucide-react'
import type { UserProfile } from '../types'

export function SettingsPage() {
  const { language } = useLanguage()
  const { user } = useAuth()
  const { data: profile, mutate: mutateProfile } = useSWR<UserProfile>(
    'user-profile',
    api.getUserProfile
  )

  return (
    <div className="space-y-6 max-w-3xl">
      {/* 页头 */}
      <div>
        <h1
          className="text-2xl font-bold flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <SettingsIcon size={24} />
          {t('settingsTitle', language)}
        </h1>
        <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
          {t('settingsDesc', language)}
        </p>
      </div>

      {/* 账户信息 */}
      <div className="binance-card p-5">
        <h2
          className="font-bold mb-4 flex items-center gap-2"
          style={{ color: '#EAECEF' }}
        >
          <Mail size={17} />
          {t('accountInfo', language)}
        </h2>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span style={{ color: '#848E9C' }}>{t('emailCol', language)}</span>
            <span className="font-mono" style={{ color: '#EAECEF' }}>
              {profile?.email ?? user?.email ?? '--'}
            </span>
          </div>
          <div className="flex justify-between">
            <span style={{ color: '#848E9C' }}>
              {t('registeredAt', language)}
            </span>
            <span className="font-mono" style={{ color: '#EAECEF' }}>
              {profile?.created_at
                ? new Date(profile.created_at).toLocaleString()
                : '--'}
            </span>
          </div>
          <div className="flex justify-between items-center">
            <span style={{ color: '#848E9C' }}>2FA</span>
            {profile?.otp_verified ? (
              <span
                className="inline-flex items-center gap-1 text-xs font-bold px-2 py-0.5 rounded"
                style={{
                  background: 'rgba(14, 203, 129, 0.1)',
                  color: '#0ECB81',
                }}
              >
                <BadgeCheck size={13} />
                {t('enabled', language)}
              </span>
            ) : (
              <span
                className="inline-flex items-center gap-1 text-xs font-bold px-2 py-0.5 rounded"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
              >
                {t('disabled', language)}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* 修改密码 */}
      <ChangePasswordCard language={language} />

      {/* 2FA 管理 */}
      <TwoFACard
        language={language}
        onChanged={mutateProfile}
        otpVerified={profile?.otp_verified ?? false}
      />
    </div>
  )
}

function ChangePasswordCard({ language }: { language: Language }) {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async () => {
    if (newPassword.length < 8) {
      toast.error(t('newPasswordTooShort', language))
      return
    }
    if (newPassword !== confirmPassword) {
      toast.error(t('settingsPasswordMismatch', language))
      return
    }
    setSaving(true)
    try {
      await api.changePassword(oldPassword, newPassword)
      toast.success(t('passwordChanged', language))
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('operationFailed', language)
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="binance-card p-5">
      <h2
        className="font-bold mb-4 flex items-center gap-2"
        style={{ color: '#EAECEF' }}
      >
        <KeyRound size={17} />
        {t('changePassword', language)}
      </h2>
      <div className="space-y-3">
        <input
          type="password"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          placeholder={t('oldPassword', language)}
          autoComplete="current-password"
          className="w-full px-3 py-2 rounded-lg text-sm outline-none"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: '#EAECEF',
          }}
        />
        <input
          type="password"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          placeholder={t('newPasswordPlaceholder8', language)}
          autoComplete="new-password"
          className="w-full px-3 py-2 rounded-lg text-sm outline-none"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: '#EAECEF',
          }}
        />
        <input
          type="password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          placeholder={t('confirmNewPassword', language)}
          autoComplete="new-password"
          className="w-full px-3 py-2 rounded-lg text-sm outline-none"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            color: '#EAECEF',
          }}
        />
        <button
          type="button"
          onClick={handleSubmit}
          disabled={saving || !oldPassword || !newPassword}
          className="px-4 py-2 rounded-lg text-sm font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center gap-1.5"
          style={{
            background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
            color: '#0B0E11',
          }}
        >
          {saving && <Loader2 size={14} className="animate-spin" />}
          {t('updatePassword', language)}
        </button>
      </div>
    </div>
  )
}

function TwoFACard({
  language,
  onChanged,
  otpVerified,
}: {
  language: Language
  onChanged: () => void
  otpVerified: boolean
}) {
  const [resetting, setResetting] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)
  const [qrUrl, setQrUrl] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [confirming, setConfirming] = useState(false)

  const handleReset = async () => {
    setResetting(true)
    try {
      const result = await api.reset2FA()
      setSecret(result.secret)
      setQrUrl(result.qr_code_url)
      toast.success(t('twofaResetDone', language))
      onChanged()
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('operationFailed', language)
      )
    } finally {
      setResetting(false)
    }
  }

  const handleConfirm = async () => {
    setConfirming(true)
    try {
      await api.confirm2FA(code)
      toast.success(t('twofaEnabled', language))
      setSecret(null)
      setQrUrl(null)
      setCode('')
      onChanged()
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('operationFailed', language)
      )
    } finally {
      setConfirming(false)
    }
  }

  return (
    <div className="binance-card p-5">
      <h2
        className="font-bold mb-1 flex items-center gap-2"
        style={{ color: '#EAECEF' }}
      >
        <ShieldCheck size={17} />
        {t('twofaManagement', language)}
      </h2>
      <p className="text-xs mb-4" style={{ color: '#848E9C' }}>
        {t('twofaDesc', language)}
      </p>

      {/* 状态 + 重置按钮 */}
      <div className="flex items-center gap-3 flex-wrap">
        <span
          className="text-xs font-bold px-2 py-1 rounded"
          style={{
            background: otpVerified
              ? 'rgba(14, 203, 129, 0.1)'
              : 'rgba(246, 70, 93, 0.1)',
            color: otpVerified ? '#0ECB81' : '#F6465D',
          }}
        >
          2FA {otpVerified ? t('enabled', language) : t('disabled', language)}
        </span>
        <button
          type="button"
          onClick={handleReset}
          disabled={resetting}
          className="px-3 py-1.5 rounded text-xs font-bold transition-all hover:scale-105 disabled:opacity-50 inline-flex items-center gap-1.5"
          style={{ background: 'rgba(255, 193, 7, 0.1)', color: '#FFC107' }}
        >
          {resetting && <Loader2 size={13} className="animate-spin" />}
          {t('resetTwofa', language)}
        </button>
      </div>

      {/* 新密钥展示 + 确认 */}
      {secret && (
        <div
          className="mt-4 p-4 rounded-lg space-y-3"
          style={{ background: '#0B0E11', border: '1px dashed #2B3139' }}
        >
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {t('twofaScanHint', language)}
          </div>
          {qrUrl && (
            <img
              src={`https://quickchart.io/qr?text=${encodeURIComponent(qrUrl)}&size=160`}
              alt="TOTP QR"
              className="rounded bg-white p-1"
              width={160}
              height={160}
            />
          )}
          <div
            className="font-mono text-xs break-all px-2 py-1.5 rounded select-all"
            style={{ background: '#1E2329', color: '#FCD535' }}
          >
            {secret}
          </div>
          <div className="flex gap-2">
            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('verificationCode', language)}
              inputMode="numeric"
              className="flex-1 px-3 py-2 rounded-lg text-sm outline-none font-mono"
              style={{
                background: '#181A20',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <button
              type="button"
              onClick={handleConfirm}
              disabled={confirming || code.length < 6}
              className="px-4 py-2 rounded-lg text-sm font-bold transition-all hover:scale-105 disabled:opacity-50 inline-flex items-center gap-1.5"
              style={{
                background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
                color: '#0B0E11',
              }}
            >
              {confirming && <Loader2 size={14} className="animate-spin" />}
              {t('confirmBtn', language)}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
