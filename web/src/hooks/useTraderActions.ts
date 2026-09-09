import { toast } from 'sonner'
import type { KeyedMutator } from 'swr'
import { api } from '../lib/api'
import { confirmToast } from '../lib/notify'
import { t, type Language } from '../i18n/translations'
import type { AIModel, CreateTraderRequest, Exchange, TraderConfigData, TraderInfo } from '../types'

interface UseTraderActionsParams {
  traders?: TraderInfo[]
  allModels: AIModel[]
  allExchanges: Exchange[]
  supportedModels: AIModel[]
  supportedExchanges: Exchange[]
  language: Language
  mutateTraders: KeyedMutator<TraderInfo[]>
  setAllModels: (models: AIModel[]) => void
  setAllExchanges: (exchanges: Exchange[]) => void
  setUserSignalSource: (source: { coinPoolUrl: string; oiTopUrl: string }) => void
  setShowCreateModal: (show: boolean) => void
  setShowEditModal: (show: boolean) => void
  setShowModelModal: (show: boolean) => void
  setShowExchangeModal: (show: boolean) => void
  setShowSignalSourceModal: (show: boolean) => void
  setEditingModel: (modelId: string | null) => void
  setEditingExchange: (exchangeId: string | null) => void
  editingTrader: TraderConfigData | null
  setEditingTrader: (trader: TraderConfigData | null) => void
}

export function useTraderActions({
  traders,
  allModels,
  allExchanges,
  supportedModels,
  language,
  mutateTraders,
  setAllModels,
  setAllExchanges,
  setUserSignalSource,
  setShowCreateModal,
  setShowEditModal,
  setShowModelModal,
  setShowExchangeModal,
  setShowSignalSourceModal,
  setEditingModel,
  setEditingExchange,
  editingTrader,
  setEditingTrader,
}: UseTraderActionsParams) {
  const isModelInUse = (modelId: string) =>
    traders?.some((trader) => trader.ai_model === modelId && trader.is_running) || false

  const isExchangeInUse = (exchangeId: string) =>
    traders?.some((trader) => trader.exchange_id === exchangeId && trader.is_running) || false

  const isModelUsedByAnyTrader = (modelId: string) =>
    traders?.some((trader) => trader.ai_model === modelId) || false

  const isExchangeUsedByAnyTrader = (exchangeId: string) =>
    traders?.some((trader) => trader.exchange_id === exchangeId) || false

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels.find((item) => item.id === data.ai_model_id)
      const exchange = allExchanges.find((item) => item.id === data.exchange_id)

      if (!model?.enabled) {
        toast.error(t('modelNotConfigured', language))
        return
      }
      if (!exchange?.enabled) {
        toast.error(t('exchangeNotConfigured', language))
        return
      }

      await toast.promise(api.createTrader(data), {
        loading: '正在创建...',
        success: '创建成功',
        error: '创建失败',
      })
      setShowCreateModal(false)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      toast.error(t('createTraderFailed', language))
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      toast.error(t('getTraderConfigFailed', language))
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    if (!editingTrader?.trader_id) return

    try {
      await toast.promise(api.updateTrader(editingTrader.trader_id, data), {
        loading: '正在保存...',
        success: '保存成功',
        error: '保存失败',
      })
      setShowEditModal(false)
      setEditingTrader(null)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      toast.error(t('updateTraderFailed', language))
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    const ok = await confirmToast(t('confirmDeleteTrader', language))
    if (!ok) return

    try {
      await toast.promise(api.deleteTrader(traderId), {
        loading: '正在删除...',
        success: '删除成功',
        error: '删除失败',
      })
      await mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      toast.error(t('deleteTraderFailed', language))
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      await toast.promise(running ? api.stopTrader(traderId) : api.startTrader(traderId), {
        loading: running ? '正在停止...' : '正在启动...',
        success: running ? '已停止' : '已启动',
        error: t('operationFailed', language),
      })
      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      toast.error(t('operationFailed', language))
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  const handleSaveModel = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      const existingModel = allModels.find((model) => model.id === modelId)
      const modelToUpdate = existingModel || supportedModels.find((model) => model.id === modelId)
      if (!modelToUpdate) {
        toast.error(t('modelNotExist', language))
        return
      }

      const updatedModels = existingModel
        ? allModels.map((model) =>
            model.id === modelId
              ? { ...model, apiKey, customApiUrl: customApiUrl || '', customModelName: customModelName || '', enabled: true }
              : model
          )
        : [...allModels, { ...modelToUpdate, apiKey, customApiUrl: customApiUrl || '', customModelName: customModelName || '', enabled: true }]

      await toast.promise(
        api.updateModelConfigs({
          models: Object.fromEntries(
            updatedModels.map((model) => [
              model.provider,
              {
                enabled: model.enabled,
                api_key: model.apiKey || '',
                custom_api_url: model.customApiUrl || '',
                custom_model_name: model.customModelName || '',
              },
            ])
          ),
        }),
        {
          loading: '正在更新模型配置...',
          success: '模型配置已更新',
          error: '更新模型配置失败',
        }
      )

      setAllModels(await api.getModelConfigs())
      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleDeleteModel = async (modelId: string) => {
    if (isModelUsedByAnyTrader(modelId)) {
      toast.error(t('cannotDeleteModelInUse', language))
      return
    }
    const ok = await confirmToast(t('confirmDeleteModel', language))
    if (!ok) return

    try {
      const updatedModels = allModels.map((model) =>
        model.id === modelId
          ? { ...model, apiKey: '', customApiUrl: '', customModelName: '', enabled: false }
          : model
      )
      await api.updateModelConfigs({
        models: Object.fromEntries(
          updatedModels.map((model) => [
            model.provider,
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      })
      setAllModels(await api.getModelConfigs())
      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to delete model config:', error)
      toast.error(t('deleteConfigFailed', language))
    }
  }

  const handleSaveExchange = async (
    exchangeId: string | null,
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    _passphrase?: string,
    testnet?: boolean,
    _hyperliquidWalletAddr?: string,
    _asterUser?: string,
    _asterSigner?: string,
    _asterPrivateKey?: string,
    _lighterWalletAddr?: string,
    _lighterPrivateKey?: string,
    _lighterApiKeyPrivateKey?: string
  ) => {
    try {
      const payload = {
        enabled: true,
        api_key: apiKey || '',
        secret_key: secretKey || '',
        testnet: testnet || false,
      }

      if (exchangeId) {
        await toast.promise(api.updateExchangeConfigsEncrypted({ exchanges: { [exchangeId]: payload } }), {
          loading: '正在更新交易所配置...',
          success: '交易所配置已更新',
          error: '更新交易所配置失败',
        })
      } else {
        await toast.promise(
          api.createExchangeEncrypted({
            exchange_type: exchangeType,
            account_name: accountName,
            ...payload,
          }),
          {
            loading: '正在创建交易所账户...',
            success: '交易所账户已创建',
            error: '创建交易所账户失败',
          }
        )
      }

      setAllExchanges(await api.getExchangeConfigs())
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleDeleteExchange = async (exchangeId: string) => {
    if (isExchangeUsedByAnyTrader(exchangeId)) {
      toast.error(t('cannotDeleteExchangeInUse', language))
      return
    }
    const ok = await confirmToast(t('confirmDeleteExchange', language))
    if (!ok) return

    try {
      await api.deleteExchange(exchangeId)
      setAllExchanges(await api.getExchangeConfigs())
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to delete exchange config:', error)
      toast.error(t('deleteExchangeConfigFailed', language))
    }
  }

  const handleSaveSignalSource = (coinPoolUrl: string, oiTopUrl: string) => {
    setUserSignalSource({ coinPoolUrl, oiTopUrl })
    setShowSignalSourceModal(false)
  }

  return {
    isModelInUse,
    isExchangeInUse,
    handleCreateTrader,
    handleEditTrader,
    handleSaveEditTrader,
    handleDeleteTrader,
    handleToggleTrader,
    handleAddModel,
    handleAddExchange,
    handleModelClick,
    handleExchangeClick,
    handleSaveModel,
    handleDeleteModel,
    handleSaveExchange,
    handleDeleteExchange,
    handleSaveSignalSource,
  }
}
