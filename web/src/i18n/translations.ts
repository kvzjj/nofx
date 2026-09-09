export type Language = 'en' | 'zh'

export const translations = {
  en: {
    // Header
    appTitle: 'AUAIEX',
    subtitle: 'Multi-AI Model Trading Platform',
    aiTraders: 'AI Traders',
    details: 'Details',
    tradingPanel: 'Trading Panel',
    competition: 'Competition',
    backtest: 'Backtest',
    running: 'RUNNING',
    stopped: 'STOPPED',
    adminMode: 'Admin Mode',
    logout: 'Logout',
    switchTrader: 'Switch Trader:',
    view: 'View',

    // Navigation
    realtimeNav: 'Live',
    configNav: 'Config',
    dashboardNav: 'Dashboard',
    strategyNav: 'Strategy',
    faqNav: 'FAQ',

    // Footer
    footerTitle: 'AUAIEX - AI Trading System',
    footerWarning: '⚠️ Trading involves risk. Use at your own discretion.',

    // Stats Cards
    totalEquity: 'Total Equity',
    availableBalance: 'Available Balance',
    totalPnL: 'Total P&L',
    positions: 'Positions',
    margin: 'Margin',
    free: 'Free',

    // Positions Table
    currentPositions: 'Current Positions',
    active: 'Active',
    symbol: 'Symbol',
    side: 'Side',
    entryPrice: 'Entry Price',
    markPrice: 'Mark Price',
    quantity: 'Quantity',
    positionValue: 'Position Value',
    leverage: 'Leverage',
    unrealizedPnL: 'Unrealized P&L',
    liqPrice: 'Liq. Price',
    long: 'LONG',
    short: 'SHORT',
    noPositions: 'No Positions',
    noActivePositions: 'No active trading positions',
    actionCol: 'Action',
    entryShort: 'Entry',
    markShort: 'Mark',
    qtyShort: 'Qty',
    valueShort: 'Value',
    levShort: 'Lev.',
    upnlShort: 'uPnL',
    liqShort: 'Liq.',
    closeBtn: 'Close',
    closePositionTitle: 'Close Position',
    confirmCloseTitle: 'Confirm Close',
    confirmBtn: 'Confirm',
    cancelBtn: 'Cancel',
    closeSuccess: 'Position closed successfully',
    closeFailed: 'Failed to close position',
    confirmClosePosition:
      'Are you sure you want to close {symbol} {side} position?',
    longPosition: 'LONG',
    shortPosition: 'SHORT',
    liveExposure: 'Live exposure',

    // Order & Fill History
    executionHistory: 'Execution history',
    orderHistory: 'Order History',
    ordersTab: 'Orders',
    fillsTab: 'Fills',
    recordsUnit: 'records',
    timeCol: 'Time',
    priceCol: 'Price',
    feeCol: 'Fee',
    statusCol: 'Status',
    realizedPnL: 'Realized P&L',
    noOrdersYet: 'No Orders Yet',
    noOrdersDesc: 'Order records will appear here after the trader executes',
    noFillsYet: 'No Fills Yet',
    noFillsDesc: 'Exchange fills will appear here after trades complete',
    protectionActive: 'Stop-loss / take-profit protection active',

    // Performance Metrics
    riskAnalytics: 'Risk & analytics',
    performanceMetrics: 'Performance Metrics',
    closedWinRate: 'Closed Win Rate',
    profitFactor: 'Profit Factor',
    grossProfit: 'Gross profit',
    grossLoss: 'Gross loss',
    avgWinLabel: 'Avg Win',
    avgLossLabel: 'Avg Loss',
    pnlBySymbol: 'Realized P&L by Symbol',

    // Export
    exportCsv: 'Export CSV',
    exportPositions: 'Export current positions as CSV',
    exportDecisions: 'Export decision logs (incl. chain of thought) as JSON',

    // Notifications page
    notificationsNav: 'Notifications',
    notificationsTitle: 'Notifications',
    notificationsDesc:
      'Deliver trade alerts (open / close / failure) to Telegram, Webhook or Email channels',
    addChannel: 'Add Channel',
    editChannel: 'Edit Channel',
    telegramChannel: 'Telegram',
    webhookChannel: 'Webhook',
    emailChannel: 'Email',
    channelName: 'Channel Name',
    channelType: 'Channel Type',
    channelNameRequired: 'Channel name is required',
    channelSaved: 'Channel saved',
    channelDeleted: 'Channel deleted',
    sendTest: 'Send Test',
    testSent: 'Test notification sent',
    testFailed: 'Test notification failed',
    noChannels: 'No notification channels',
    noChannelsDesc:
      'Add a Telegram / Webhook / Email channel to receive trade alerts',
    deliveryLogs: 'Delivery Logs',
    noDeliveryLogs: 'No delivery logs yet',
    eventCol: 'Event',
    messageCol: 'Message',
    channelNameCol: 'Channel',
    failedCol: 'Failed',
    secretField: 'secret',
    channelHelp:
      'Telegram: create a bot via @BotFather, then message it (or add it to a group) and use the chat ID. Webhook receives JSON POST with HMAC-SHA256 signature header X-Auaiex-Signature when a secret is set. Email uses your own SMTP server.',

    // Settings page
    settingsNav: 'Settings',
    settingsTitle: 'Account Settings',
    settingsDesc: 'Manage your profile, password and two-factor authentication',
    accountInfo: 'Account Information',
    emailCol: 'Email',
    registeredAt: 'Registered at',
    changePassword: 'Change Password',
    oldPassword: 'Current password',
    newPasswordPlaceholder8: 'New password (min 8 chars)',
    confirmNewPassword: 'Confirm new password',
    updatePassword: 'Update Password',
    newPasswordTooShort: 'New password must be at least 8 characters',
    settingsPasswordMismatch: 'New passwords do not match',
    passwordChanged: 'Password updated successfully',
    twofaManagement: 'Two-Factor Authentication',
    twofaDesc:
      'Reset generates a new TOTP secret. You must re-scan the QR code with your authenticator app and confirm a code to re-enable 2FA. Login requires 2FA when enabled.',
    resetTwofa: 'Reset 2FA Secret',
    twofaResetDone: 'New secret generated, please scan and confirm',
    twofaScanHint:
      'Scan this QR code with your authenticator app (e.g. Google Authenticator), then enter the 6-digit code to confirm:',
    twofaEnabled: '2FA enabled successfully',
    verificationCode: '6-digit code',

    // Admin page
    adminNav: 'Admin',
    adminTitle: 'Admin Panel',
    adminDesc:
      'System health, user overview and global trading activity (admin account only)',
    adminUsersTotal: 'Users',
    adminVerified: 'Verified',
    adminTradersTotal: 'Traders',
    adminTradersRunning: 'Running',
    adminMemory: 'Memory',
    adminGoroutines: 'Goroutines',
    adminUserList: 'Users',
    adminTraderCount: 'Traders',
    adminRunningCount: 'Running',
    adminNoUsers: 'No users',
    adminRecentActivity: 'Recent Activity (all traders)',

    // Manual order
    manualOrder: 'Manual Order',
    confirmManualOrder:
      'Submit {action} order for {symbol}? This goes through the same risk controls and protection orders as AI decisions.',
    manualOrderSuccess: 'Manual order executed',
    manualOrderFailed: 'Manual order failed',
    stopLossPctLabel: 'Stop loss % (from entry)',
    takeProfitPctLabel: 'Take profit % (from entry)',
    submitOrder: 'Submit Order',
    manualOrderHelp:
      'Position size and leverage are computed by the backend risk engine (same as AI trades); stop loss / take profit percentages are converted to absolute prices at the current market price.',
    manualCloseHelp:
      'The entire position for this symbol and direction will be closed at market price.',

    // Batch trader management
    selectAll: 'Select all',
    batchStart: 'Batch Start',
    batchStop: 'Batch Stop',

    // Theme
    switchToLight: 'Switch to light theme',
    switchToDark: 'Switch to dark theme',

    // Risk controls
    closeAll: 'Close All',
    confirmCloseAll:
      'Are you sure you want to close ALL {count} positions? This cannot be undone.',
    closeAllSuccess: 'All positions closed',
    closeAllPartial: '{success}/{total} positions closed, {failed} failed',
    distanceToLiq: 'To Liq.',
    liqWarningTitle: 'Liquidation risk',
    stopTrading: 'Stop Trading',
    confirmStopTrading: 'Stop auto trading for this trader?',
    traderStopped: 'Trading stopped',
    stopTradingFailed: 'Failed to stop trading',

    // Trading statistics
    tradingStats: 'Trading Stats',
    cycleSuccessRate: 'Cycle Success Rate',
    totalCyclesLabel: 'Total Cycles',
    openTradesTotal: 'Positions Opened',
    closeTradesTotal: 'Positions Closed',
    maxDrawdown: 'Max Drawdown',
    noStatsData: 'No statistics yet',

    // Browser notifications
    enableNotifications: 'Enable notifications',
    notificationsEnabled: 'Notifications on',
    notificationDenied: 'Notification permission was denied by the browser',
    notifPositionOpened: '{trader} opened {side} {symbol}',
    notifPositionClosed: '{trader} closed {side} {symbol}',
    notifLiqRisk: '{symbol} is only {pct}% away from liquidation',
    notifDecisionError: '{trader} decision cycle failed',

    // Connection status
    connectionLost: 'Connection lost — showing data from {time}',
    dataStale: 'Data may be outdated — last update {time}',
    liveLabel: 'Live',

    // Dashboard hero
    autoTradingActive: 'Auto trading active',
    traderOverview: 'Trader overview',
    cyclesCount: '{count} cycles',
    updatedAt: 'Updated {time}',
    noStrategy: 'No Strategy',

    // Recent Decisions
    recentDecisions: 'Recent Decisions',
    lastCycles: 'Last {count} trading cycles',
    noDecisionsYet: 'No Decisions Yet',
    aiDecisionsWillAppear: 'AI trading decisions will appear here',
    cycle: 'Cycle',
    success: 'Success',
    failed: 'Failed',
    inputPrompt: 'Input Prompt',
    aiThinking: 'AI Chain of Thought',
    collapse: 'Collapse',
    expand: 'Expand',

    // Equity Chart
    accountEquityCurve: 'Account Equity Curve',
    noHistoricalData: 'No Historical Data',
    dataWillAppear: 'Equity curve will appear after running a few cycles',
    initialBalance: 'Initial Balance',
    currentEquity: 'Current Equity',
    historicalCycles: 'Historical Cycles',
    displayRange: 'Display Range',
    recent: 'Recent',
    allData: 'All Data',
    cycles: 'Cycles',

    // Comparison Chart
    comparisonMode: 'Comparison Mode',
    dataPoints: 'Data Points',
    currentGap: 'Current Gap',
    count: '{count} pts',

    // TradingView Chart
    marketChart: 'Market Chart',
    viewChart: 'Click to view chart',
    enterSymbol: 'Enter symbol...',
    popularSymbols: 'Popular Symbols',
    fullscreen: 'Fullscreen',
    exitFullscreen: 'Exit Fullscreen',

    // Backtest Page
    backtestPage: {
      title: 'Backtest Lab',
      subtitle:
        'Pick a model + time range to replay the full AI decision loop.',
      start: 'Start Backtest',
      starting: 'Starting...',
      quickRanges: {
        h24: '24h',
        d3: '3d',
        d7: '7d',
      },
      actions: {
        pause: 'Pause',
        resume: 'Resume',
        stop: 'Stop',
      },
      states: {
        running: 'Running',
        paused: 'Paused',
        completed: 'Completed',
        failed: 'Failed',
        liquidated: 'Liquidated',
      },
      form: {
        aiModelLabel: 'AI Model',
        selectAiModel: 'Select AI model',
        providerLabel: 'Provider',
        statusLabel: 'Status',
        enabled: 'Enabled',
        disabled: 'Disabled',
        noModelWarning:
          'Please add and enable an AI model on the Model Config page first.',
        runIdLabel: 'Run ID',
        runIdPlaceholder: 'Leave blank to auto-generate',
        decisionTfLabel: 'Decision TF',
        cadenceLabel: 'Decision cadence (bars)',
        timeRangeLabel: 'Time range',
        symbolsLabel: 'Symbols (comma-separated)',
        customTfPlaceholder: 'Custom TFs (comma separated, e.g. 2h,6h)',
        initialBalanceLabel: 'Initial balance (USDT)',
        feeLabel: 'Fee (bps)',
        slippageLabel: 'Slippage (bps)',
        btcEthLeverageLabel: 'BTC/ETH leverage (x)',
        altcoinLeverageLabel: 'Altcoin leverage (x)',
        fillPolicies: {
          nextOpen: 'Next open',
          barVwap: 'Bar VWAP',
          midPrice: 'Mid price',
        },
        promptPresets: {
          baseline: 'Baseline',
          aggressive: 'Aggressive',
          conservative: 'Conservative',
          scalping: 'Scalping',
        },
        cacheAiLabel: 'Reuse AI cache',
        replayOnlyLabel: 'Replay only',
        overridePromptLabel: 'Use only custom prompt',
        customPromptLabel: 'Custom prompt (optional)',
        customPromptPlaceholder:
          'Append or fully customize the strategy prompt',
      },
      runList: {
        title: 'Runs',
        count: 'Total {count} records',
      },
      compare: {
        title: 'Run Comparison',
        desc: 'Select up to {max} finished runs to overlay normalized return curves and compare metrics (best value highlighted)',
        clear: 'Clear selection',
        noFinished: 'No finished runs available for comparison yet',
        metric: 'Metric',
        totalReturn: 'Total Return',
        maxDrawdown: 'Max Drawdown',
        sharpe: 'Sharpe Ratio',
        profitFactor: 'Profit Factor',
        winRate: 'Win Rate',
        trades: 'Trades',
      },
      filters: {
        allStates: 'All states',
        searchPlaceholder: 'Run ID / label',
      },
      tableHeaders: {
        runId: 'Run ID',
        label: 'Label',
        state: 'State',
        progress: 'Progress',
        equity: 'Equity',
        lastError: 'Last Error',
        updated: 'Updated',
      },
      emptyStates: {
        noRuns: 'No runs yet',
        selectRun: 'Select a run to view details',
      },
      detail: {
        tfAndSymbols: 'TF: {tf} · Symbols {count}',
        labelPlaceholder: 'Label note',
        saveLabel: 'Save',
        deleteLabel: 'Delete',
        exportLabel: 'Export',
        errorLabel: 'Error',
      },
      toasts: {
        selectModel: 'Please select an AI model first.',
        modelDisabled: 'AI model {name} is disabled.',
        invalidRange: 'End time must be later than start time.',
        startSuccess: 'Backtest {id} started.',
        startFailed: 'Failed to start. Please try again later.',
        actionSuccess: '{action} {id} succeeded.',
        actionFailed: 'Operation failed. Please try again later.',
        labelSaved: 'Label updated.',
        labelFailed: 'Failed to update label.',
        confirmDelete: 'Delete backtest {id}? This action cannot be undone.',
        deleteSuccess: 'Backtest record deleted.',
        deleteFailed: 'Failed to delete. Please try again later.',
        traceFailed: 'Failed to fetch AI trace.',
        exportSuccess: 'Exported data for {id}.',
        exportFailed: 'Failed to export.',
      },
      aiTrace: {
        title: 'AI Trace',
        clear: 'Clear',
        cyclePlaceholder: 'Cycle',
        fetch: 'Fetch',
        prompt: 'Prompt',
        cot: 'Chain of thought',
        output: 'Output',
        cycleTag: 'Cycle #{cycle}',
      },
      decisionTrail: {
        title: 'AI Decision Trail',
        subtitle: 'Showing last {count} cycles',
        empty: 'No records yet',
        emptyHint:
          'The AI thought & execution log will appear once the run starts.',
      },
      charts: {
        equityTitle: 'Equity Curve',
        equityEmpty: 'No data yet',
      },
      metrics: {
        title: 'Metrics',
        totalReturn: 'Total Return %',
        maxDrawdown: 'Max Drawdown %',
        sharpe: 'Sharpe',
        profitFactor: 'Profit Factor',
        pending: 'Calculating...',
        realized: 'Realized PnL',
        unrealized: 'Unrealized PnL',
      },
      trades: {
        title: 'Trade Events',
        headers: {
          time: 'Time',
          symbol: 'Symbol',
          action: 'Action',
          qty: 'Qty',
          leverage: 'Leverage',
          pnl: 'PnL',
        },
        empty: 'No trades yet',
      },
      metadata: {
        title: 'Metadata',
        created: 'Created',
        updated: 'Updated',
        processedBars: 'Processed Bars',
        maxDrawdown: 'Max DD',
        liquidated: 'Liquidated',
        yes: 'Yes',
        no: 'No',
      },
    },

    // Competition Page
    aiCompetition: 'AI Competition',
    traders: 'traders',
    liveBattle: 'Live Battle',
    realTimeBattle: 'Real-time Battle',
    leader: 'Leader',
    leaderboard: 'Leaderboard',
    live: 'LIVE',
    realTime: 'LIVE',
    performanceComparison: 'Performance Comparison',
    realTimePnL: 'Real-time PnL %',
    realTimePnLPercent: 'Real-time PnL %',
    headToHead: 'Head-to-Head Battle',
    leadingBy: 'Leading by {gap}%',
    behindBy: 'Behind by {gap}%',
    equity: 'Equity',
    pnl: 'P&L',
    pos: 'Pos',

    // AI Traders Management
    manageAITraders: 'Manage your AI trading bots',
    aiModels: 'AI Models',
    exchanges: 'Exchanges',
    createTrader: 'Create Trader',
    modelConfiguration: 'Model Configuration',
    configured: 'Configured',
    notConfigured: 'Not Configured',
    currentTraders: 'Current Traders',
    noTraders: 'No AI Traders',
    createFirstTrader: 'Create your first AI trader to get started',
    dashboardEmptyTitle: "Let's Get Started!",
    dashboardEmptyDescription:
      'Create your first AI trader to automate your trading strategy. Connect an exchange, choose an AI model, and start trading in minutes!',
    goToTradersPage: 'Create Your First Trader',
    configureModelsFirst: 'Please configure AI models first',
    configureExchangesFirst: 'Please configure exchanges first',
    configureModelsAndExchangesFirst:
      'Please configure AI models and exchanges first',
    modelNotConfigured: 'Selected model is not configured',
    exchangeNotConfigured: 'Selected exchange is not configured',
    confirmDeleteTrader: 'Are you sure you want to delete this trader?',
    status: 'Status',
    start: 'Start',
    stop: 'Stop',
    createNewTrader: 'Create New AI Trader',
    selectAIModel: 'Select AI Model',
    selectExchange: 'Select Exchange',
    traderName: 'Trader Name',
    enterTraderName: 'Enter trader name',
    cancel: 'Cancel',
    create: 'Create',
    configureAIModels: 'Configure AI Models',
    configureExchanges: 'Configure Exchanges',
    aiScanInterval: 'AI Scan Decision Interval (minutes)',
    scanIntervalRecommend: 'Recommended: 3-10 minutes',
    useTestnet: 'Use Testnet',
    enabled: 'Enabled',
    save: 'Save',

    // AI Model Configuration
    officialAPI: 'Official API',
    customAPI: 'Custom API',
    apiKey: 'API Key',
    customAPIURL: 'Custom API URL',
    enterAPIKey: 'Enter API Key',
    enterCustomAPIURL: 'Enter custom API endpoint URL',
    useOfficialAPI: 'Use official API service',
    useCustomAPI: 'Use custom API endpoint',

    // Exchange Configuration
    secretKey: 'Secret Key',
    privateKey: 'Private Key',
    walletAddress: 'Wallet Address',
    user: 'User',
    signer: 'Signer',
    passphrase: 'Passphrase',
    enterPrivateKey: 'Enter Private Key',
    enterWalletAddress: 'Enter Wallet Address',
    enterUser: 'Enter User',
    enterSigner: 'Enter Signer Address',
    enterSecretKey: 'Enter Secret Key',
    enterPassphrase: 'Enter Passphrase',
    hyperliquidPrivateKeyDesc:
      'Hyperliquid uses private key for trading authentication',
    hyperliquidWalletAddressDesc:
      'Wallet address corresponding to the private key',
    // Hyperliquid Agent Wallet (New Security Model)
    hyperliquidAgentWalletTitle: 'Hyperliquid Agent Wallet Configuration',
    hyperliquidAgentWalletDesc:
      'Use Agent Wallet for secure trading: Agent wallet signs transactions (balance ~0), Main wallet holds funds (never expose private key)',
    hyperliquidAgentPrivateKey: 'Agent Private Key',
    enterHyperliquidAgentPrivateKey: 'Enter Agent wallet private key',
    hyperliquidAgentPrivateKeyDesc:
      'Agent wallet private key for signing transactions (keep balance near 0 for security)',
    hyperliquidMainWalletAddress: 'Main Wallet Address',
    enterHyperliquidMainWalletAddress: 'Enter Main wallet address',
    hyperliquidMainWalletAddressDesc:
      'Main wallet address that holds your trading funds (never expose its private key)',
    // Aster API Pro Configuration
    asterApiProTitle: 'Aster API Pro Wallet Configuration',
    asterApiProDesc:
      'Use API Pro wallet for secure trading: API wallet signs transactions, main wallet holds funds (never expose main wallet private key)',
    asterUserDesc:
      'Main wallet address - The EVM wallet address you use to log in to Aster (Note: Only EVM wallets are supported)',
    asterSignerDesc:
      'API Pro wallet address (0x...) - Generate from https://www.asterdex.com/en/api-wallet',
    asterPrivateKeyDesc:
      'API Pro wallet private key - Get from https://www.asterdex.com/en/api-wallet (only used locally for signing, never transmitted)',
    asterUsdtWarning:
      'Important: Aster only tracks USDT balance. Please ensure you use USDT as margin currency to avoid P&L calculation errors caused by price fluctuations of other assets (BNB, ETH, etc.)',
    asterUserLabel: 'Main Wallet Address',
    asterSignerLabel: 'API Pro Wallet Address',
    asterPrivateKeyLabel: 'API Pro Wallet Private Key',
    enterAsterUser: 'Enter main wallet address (0x...)',
    enterAsterSigner: 'Enter API Pro wallet address (0x...)',
    enterAsterPrivateKey: 'Enter API Pro wallet private key',

    // LIGHTER Configuration
    lighterWalletAddress: 'L1 Wallet Address',
    lighterPrivateKey: 'L1 Private Key',
    lighterApiKeyPrivateKey: 'API Key Private Key',
    enterLighterWalletAddress: 'Enter Ethereum wallet address (0x...)',
    enterLighterPrivateKey: 'Enter L1 private key (32 bytes)',
    enterLighterApiKeyPrivateKey:
      'Enter API Key private key (40 bytes, optional)',
    lighterWalletAddressDesc:
      'Your Ethereum wallet address for account identification',
    lighterPrivateKeyDesc:
      'L1 private key for account identification (32-byte ECDSA key)',
    lighterApiKeyPrivateKeyDesc:
      'API Key private key for transaction signing (40-byte Poseidon2 key)',
    lighterApiKeyOptionalNote:
      'Without API Key, system will use limited V1 mode',
    lighterV1Description:
      'Basic Mode - Limited functionality, testing framework only',
    lighterV2Description:
      'Full Mode - Supports Poseidon2 signing and real trading',
    lighterPrivateKeyImported: 'LIGHTER private key imported',

    // Exchange names
    hyperliquidExchangeName: 'Hyperliquid',
    asterExchangeName: 'Aster DEX',

    // Secure input
    secureInputButton: 'Secure Input',
    secureInputReenter: 'Re-enter Securely',
    secureInputClear: 'Clear',
    secureInputHint:
      'Captured via secure two-step input. Use "Re-enter Securely" to update this value.',

    // Two Stage Key Modal
    twoStageModalTitle: 'Secure Key Input',
    twoStageModalDescription:
      'Use a two-step flow to enter your {length}-character private key safely.',
    twoStageStage1Title: 'Step 1 · Enter the first half',
    twoStageStage1Placeholder: 'First 32 characters (include 0x if present)',
    twoStageStage1Hint:
      'Continuing copies an obfuscation string to your clipboard as a diversion.',
    twoStageStage1Error: 'Please enter the first part before continuing.',
    twoStageNext: 'Next',
    twoStageProcessing: 'Processing…',
    twoStageCancel: 'Cancel',
    twoStageStage2Title: 'Step 2 · Enter the rest',
    twoStageStage2Placeholder: 'Remaining characters of your private key',
    twoStageStage2Hint:
      'Paste the obfuscation string somewhere neutral, then finish entering your key.',
    twoStageClipboardSuccess:
      'Obfuscation string copied. Paste it into any text field once before completing.',
    twoStageClipboardReminder:
      'Remember to paste the obfuscation string before submitting to avoid clipboard leaks.',
    twoStageClipboardManual:
      'Automatic copy failed. Copy the obfuscation string below manually.',
    twoStageBack: 'Back',
    twoStageSubmit: 'Confirm',
    twoStageInvalidFormat:
      'Invalid private key format. Expected {length} hexadecimal characters (optional 0x prefix).',
    testnetDescription:
      'Enable to connect to exchange test environment for simulated trading',
    securityWarning: 'Security Warning',
    saveConfiguration: 'Save Configuration',

    // Trader Configuration
    positionMode: 'Position Mode',
    crossMarginMode: 'Cross Margin',
    isolatedMarginMode: 'Isolated Margin',
    crossMarginDescription:
      'Cross margin: All positions share account balance as collateral',
    isolatedMarginDescription:
      'Isolated margin: Each position manages collateral independently, risk isolation',
    leverageConfiguration: 'Leverage Configuration',
    btcEthLeverage: 'BTC/ETH Leverage',
    altcoinLeverage: 'Altcoin Leverage',
    leverageRecommendation:
      'Recommended: BTC/ETH 5-10x, Altcoins 3-5x for risk control',
    tradingSymbols: 'Trading Symbols',
    tradingSymbolsPlaceholder:
      'Enter symbols, comma separated (e.g., BTCUSDT,ETHUSDT,SOLUSDT)',
    selectSymbols: 'Select Symbols',
    selectTradingSymbols: 'Select Trading Symbols',
    selectedSymbolsCount: 'Selected {count} symbols',
    clearSelection: 'Clear All',
    confirmSelection: 'Confirm',
    tradingSymbolsDescription:
      'Empty = use default symbols. Must end with USDT (e.g., BTCUSDT, ETHUSDT)',
    btcEthLeverageValidation: 'BTC/ETH leverage must be between 1-50x',
    altcoinLeverageValidation: 'Altcoin leverage must be between 1-20x',
    invalidSymbolFormat: 'Invalid symbol format: {symbol}, must end with USDT',

    // System Prompt Templates
    systemPromptTemplate: 'System Prompt Template',
    promptTemplateDefault: 'Default Stable',
    promptTemplateAdaptive: 'Conservative Strategy',
    promptTemplateAdaptiveRelaxed: 'Aggressive Strategy',
    promptTemplateHansen: 'Hansen Strategy',
    promptTemplateNof1: 'NoF1 English Framework',
    promptTemplateTaroLong: 'Taro Long Position',
    promptDescDefault: '📊 Default Stable Strategy',
    promptDescDefaultContent:
      'Maximize Sharpe ratio, balanced risk-reward, suitable for beginners and stable long-term trading',
    promptDescAdaptive: '🛡️ Conservative Strategy (v6.0.0)',
    promptDescAdaptiveContent:
      'Strict risk control, BTC mandatory confirmation, high win rate priority, suitable for conservative traders',
    promptDescAdaptiveRelaxed: '⚡ Aggressive Strategy (v6.0.0)',
    promptDescAdaptiveRelaxedContent:
      'High-frequency trading, BTC optional confirmation, pursue trading opportunities, suitable for volatile markets',
    promptDescHansen: '🎯 Hansen Strategy',
    promptDescHansenContent:
      'Hansen custom strategy, maximize Sharpe ratio, for professional traders',
    promptDescNof1: '🌐 NoF1 English Framework',
    promptDescNof1Content:
      'Hyperliquid exchange specialist, English prompts, maximize risk-adjusted returns',
    promptDescTaroLong: '📈 Taro Long Position Strategy',
    promptDescTaroLongContent:
      'Data-driven decisions, multi-dimensional validation, continuous learning evolution, long position specialist',

    // Loading & Error
    loading: 'Loading...',

    // AI Traders Page - Additional
    inUse: 'In Use',
    noModelsConfigured: 'No configured AI models',
    noExchangesConfigured: 'No configured exchanges',
    signalSource: 'Signal Source',
    signalSourceConfig: 'Signal Source Configuration',
    coinPoolDescription:
      'API endpoint for coin pool data, leave blank to disable this signal source',
    oiTopDescription:
      'API endpoint for open interest rankings, leave blank to disable this signal source',
    information: 'Information',
    signalSourceInfo1:
      '• Signal source configuration is per-user, each user can set their own URLs',
    signalSourceInfo2:
      '• When creating traders, you can choose whether to use these signal sources',
    signalSourceInfo3:
      '• Configured URLs will be used to fetch market data and trading signals',
    editAIModel: 'Edit AI Model',
    addAIModel: 'Add AI Model',
    confirmDeleteModel:
      'Are you sure you want to delete this AI model configuration?',
    cannotDeleteModelInUse:
      'Cannot delete this AI model because it is being used by traders',
    tradersUsing: 'Traders using this configuration',
    pleaseDeleteTradersFirst:
      'Please delete or reconfigure these traders first',
    selectModel: 'Select AI Model',
    pleaseSelectModel: 'Please select a model',
    customBaseURL: 'Base URL (Optional)',
    customBaseURLPlaceholder:
      'Custom API base URL, e.g.: https://api.openai.com/v1',
    leaveBlankForDefault: 'Leave blank to use default API address',
    modelConfigInfo1:
      '• For official API, only API Key is required, leave other fields blank',
    modelConfigInfo2:
      '• Custom Base URL and Model Name only needed for third-party proxies',
    modelConfigInfo3: '• API Key is encrypted and stored securely',
    defaultModel: 'Default model',
    applyApiKey: 'Apply API Key',
    kimiApiNote:
      'Kimi requires API Key from international site (moonshot.ai), China region keys are not compatible',
    leaveBlankForDefaultModel: 'Leave blank to use default model',
    customModelName: 'Model Name (Optional)',
    customModelNamePlaceholder: 'e.g.: deepseek-chat, qwen3-max, gpt-4o',
    saveConfig: 'Save Configuration',
    editExchange: 'Edit Exchange',
    addExchange: 'Add Exchange',
    confirmDeleteExchange:
      'Are you sure you want to delete this exchange configuration?',
    cannotDeleteExchangeInUse:
      'Cannot delete this exchange because it is being used by traders',
    pleaseSelectExchange: 'Please select an exchange',
    exchangeConfigWarning1:
      '• API keys will be encrypted, recommend using read-only or futures trading permissions',
    exchangeConfigWarning2:
      '• Do not grant withdrawal permissions to ensure fund security',
    exchangeConfigWarning3:
      '• After deleting configuration, related traders will not be able to trade',
    edit: 'Edit',
    viewGuide: 'View Guide',
    binanceSetupGuide: 'Binance Setup Guide',
    closeGuide: 'Close',
    whitelistIP: 'Whitelist IP',
    whitelistIPDesc: 'Binance requires adding server IP to API whitelist',
    serverIPAddresses: 'Server IP Addresses',
    copyIP: 'Copy',
    ipCopied: 'IP Copied',
    copyIPFailed: 'Failed to copy IP address. Please copy manually',
    loadingServerIP: 'Loading server IP...',

    // Error Messages
    createTraderFailed: 'Failed to create trader',
    getTraderConfigFailed: 'Failed to get trader configuration',
    modelConfigNotExist: 'Model configuration does not exist or is not enabled',
    exchangeConfigNotExist:
      'Exchange configuration does not exist or is not enabled',
    updateTraderFailed: 'Failed to update trader',
    deleteTraderFailed: 'Failed to delete trader',
    operationFailed: 'Operation failed',
    deleteConfigFailed: 'Failed to delete configuration',
    modelNotExist: 'Model does not exist',
    saveConfigFailed: 'Failed to save configuration',
    exchangeNotExist: 'Exchange does not exist',
    deleteExchangeConfigFailed: 'Failed to delete exchange configuration',
    saveSignalSourceFailed: 'Failed to save signal source configuration',
    encryptionFailed: 'Failed to encrypt sensitive data',

    // Login & Register
    login: 'Sign In',
    register: 'Sign Up',
    username: 'Username',
    email: 'Email',
    password: 'Password',
    confirmPassword: 'Confirm Password',
    usernamePlaceholder: 'your username',
    emailPlaceholder: 'your@email.com',
    passwordPlaceholder: 'Enter your password',
    confirmPasswordPlaceholder: 'Re-enter your password',
    passwordRequirements: 'Password requirements',
    passwordRuleMinLength: 'Minimum 8 characters',
    passwordRuleUppercase: 'At least 1 uppercase letter',
    passwordRuleLowercase: 'At least 1 lowercase letter',
    passwordRuleNumber: 'At least 1 number',
    passwordRuleSpecial: 'At least 1 special character (@#$%!&*?)',
    passwordRuleMatch: 'Passwords match',
    passwordNotMeetRequirements:
      'Password does not meet the security requirements',
    otpPlaceholder: '000000',
    loginTitle: 'Sign in to your account',
    registerTitle: 'Create a new account',
    loginButton: 'Sign In',
    registerButton: 'Sign Up',
    back: 'Back',
    noAccount: "Don't have an account?",
    hasAccount: 'Already have an account?',
    registerNow: 'Sign up now',
    loginNow: 'Sign in now',
    forgotPassword: 'Forgot password?',
    rememberMe: 'Remember me',
    otpCode: 'OTP Code',
    resetPassword: 'Reset Password',
    resetPasswordTitle: 'Reset your password',
    newPassword: 'New Password',
    newPasswordPlaceholder: 'Enter new password (at least 6 characters)',
    resetPasswordButton: 'Reset Password',
    resetPasswordSuccess:
      'Password reset successful! Please login with your new password',
    resetPasswordFailed: 'Password reset failed',
    backToLogin: 'Back to Login',
    scanQRCode: 'Scan QR Code',
    enterOTPCode: 'Enter 6-digit OTP code',
    verifyOTP: 'Verify OTP',
    setupTwoFactor: 'Set up two-factor authentication',
    setupTwoFactorDesc:
      'Follow the steps below to secure your account with Google Authenticator',
    scanQRCodeInstructions:
      'Scan this QR code with Google Authenticator or Authy',
    otpSecret: 'Or enter this secret manually:',
    qrCodeHint: 'QR code (if scanning fails, use the secret below):',
    authStep1Title: 'Step 1: Install Google Authenticator',
    authStep1Desc:
      'Download and install Google Authenticator from your app store',
    authStep2Title: 'Step 2: Add account',
    authStep2Desc: 'Tap "+", then choose "Scan QR code" or "Enter a setup key"',
    authStep3Title: 'Step 3: Verify setup',
    authStep3Desc: 'After setup, continue to enter the 6-digit code',
    setupCompleteContinue: 'I have completed setup, continue',
    copy: 'Copy',
    completeRegistration: 'Complete Registration',
    completeRegistrationSubtitle: 'to complete registration',
    loginSuccess: 'Login successful',
    registrationSuccess: 'Registration successful',
    loginFailed: 'Login failed. Please check your email and password.',
    registrationFailed: 'Registration failed. Please try again.',
    verificationFailed:
      'OTP verification failed. Please check the code and try again.',
    sessionExpired: 'Session expired, please login again',
    invalidCredentials: 'Invalid email or password',
    weak: 'Weak',
    medium: 'Medium',
    strong: 'Strong',
    passwordStrength: 'Password strength',
    passwordStrengthHint:
      'Use at least 8 characters with mix of letters, numbers and symbols',
    passwordMismatch: 'Passwords do not match',
    emailRequired: 'Email is required',
    passwordRequired: 'Password is required',
    invalidEmail: 'Invalid email format',
    passwordTooShort: 'Password must be at least 6 characters',

    // Landing Page
    features: 'Features',
    howItWorks: 'How it Works',
    community: 'Community',
    language: 'Language',
    loggedInAs: 'Logged in as',
    exitLogin: 'Sign Out',
    signIn: 'Sign In',
    signUp: 'Sign Up',
    registrationClosed: 'Registration Closed',
    registrationClosedMessage:
      'User registration is currently disabled. Please contact the administrator for access.',

    // Hero Section
    githubStarsInDays: '2.5K+ GitHub Stars in 3 days',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'AUAIEX is the future standard for AI trading — an open, community-driven agentic trading OS. Supporting Binance and OKX, self-hosted, multi-agent competition, let AI automatically make decisions, execute and optimize trades for you.',
    poweredBy:
      'Powered by Binance and OKX, strategically invested by Amber.ac.',

    // Landing Page CTA
    readyToDefine: 'Ready to define the future of AI trading?',
    startWithCrypto:
      'Starting with crypto markets, expanding to TradFi. AUAIEX is the infrastructure of AgentFi.',
    getStartedNow: 'Get Started Now',
    viewSourceCode: 'View Source Code',

    // Features Section
    coreFeatures: 'Core Features',
    whyChooseAuaiex: 'Why Choose AUAIEX?',
    openCommunityDriven:
      'Open source, transparent, community-driven AI trading OS',
    openSourceSelfHosted: '100% Open Source & Self-Hosted',
    openSourceDesc:
      'Your framework, your rules. Non-black box, supports custom prompts and multi-models.',
    openSourceFeatures1: 'Fully open source code',
    openSourceFeatures2: 'Self-hosting deployment support',
    openSourceFeatures3: 'Custom AI prompts',
    openSourceFeatures4: 'Multi-model support (DeepSeek, Qwen)',
    multiAgentCompetition: 'Multi-Agent Intelligent Competition',
    multiAgentDesc:
      'AI strategies battle at high speed in sandbox, survival of the fittest, achieving strategy evolution.',
    multiAgentFeatures1: 'Multiple AI agents running in parallel',
    multiAgentFeatures2: 'Automatic strategy optimization',
    multiAgentFeatures3: 'Sandbox security testing',
    multiAgentFeatures4: 'Cross-market strategy porting',
    secureReliableTrading: 'Secure and Reliable Trading',
    secureDesc:
      'Enterprise-grade security, complete control over your funds and trading strategies.',
    secureFeatures1: 'Local private key management',
    secureFeatures2: 'Fine-grained API permission control',
    secureFeatures3: 'Real-time risk monitoring',
    secureFeatures4: 'Trading log auditing',

    // About Section
    aboutAuaiex: 'About AUAIEX',
    whatIsAuaiex: 'What is AUAIEX?',
    auaiexNotAnotherBot:
      "AUAIEX is not another trading bot, but the 'Linux' of AI trading —",
    auaiexDescription1:
      'a transparent, trustworthy open source OS that provides a unified',
    auaiexDescription2:
      "'decision-risk-execution' layer, supporting all asset classes.",
    auaiexDescription3:
      'Starting with crypto markets (24/7, high volatility perfect testing ground), future expansion to stocks, futures, forex. Core: open architecture, AI',
    auaiexDescription4:
      'Darwinism (multi-agent self-competition, strategy evolution), CodeFi',
    auaiexDescription5:
      'flywheel (developers get point rewards for PR contributions).',
    youFullControl: 'You 100% Control',
    fullControlDesc: 'Complete control over AI prompts and funds',
    startupMessages1: 'Starting automated trading system...',
    startupMessages2: 'API server started on port 8080',
    startupMessages3: 'Web console http://localhost:3000',

    // How It Works Section
    howToStart: 'How to Get Started with AUAIEX',
    fourSimpleSteps:
      'Four simple steps to start your AI automated trading journey',
    step1Title: 'Clone GitHub Repository',
    step1Desc:
      'git clone https://github.com/tinkle-community/auaiex and switch to dev branch to test new features.',
    step2Title: 'Configure Environment',
    step2Desc:
      'Frontend setup for exchange APIs (Binance and OKX), AI models and custom prompts.',
    step3Title: 'Deploy & Run',
    step3Desc:
      'One-click Docker deployment, start AI agents. Note: High-risk market, only test with money you can afford to lose.',
    step4Title: 'Optimize & Contribute',
    step4Desc:
      'Monitor trading, submit PRs to improve framework. Join Telegram to share strategies.',
    importantRiskWarning: 'Important Risk Warning',
    riskWarningText:
      'Dev branch is unstable, do not use funds you cannot afford to lose. AUAIEX is non-custodial, no official strategies. Trading involves risks, invest carefully.',

    // Community Section (testimonials are kept as-is since they are quotes)

    // Footer Section
    futureStandardAI: 'The future standard of AI trading',
    links: 'Links',
    resources: 'Resources',
    documentation: 'Documentation',
    supporters: 'Supporters',
    strategicInvestment: '(Strategic Investment)',

    // Login Modal
    accessAuaiexPlatform: 'Access AUAIEX Platform',
    loginRegisterPrompt:
      'Please login or register to access the full AI trading platform',
    registerNewAccount: 'Register New Account',

    // Candidate Coins Warnings
    candidateCoins: 'Candidate Coins',
    candidateCoinsZeroWarning: 'Candidate Coins Count is 0',
    possibleReasons: 'Possible Reasons:',
    coinPoolApiNotConfigured:
      'Coin pool API not configured or inaccessible (check signal source settings)',
    apiConnectionTimeout: 'API connection timeout or returned empty data',
    noCustomCoinsAndApiFailed:
      'No custom coins configured and API fetch failed',
    solutions: 'Solutions:',
    setCustomCoinsInConfig: 'Set custom coin list in trader configuration',
    orConfigureCorrectApiUrl: 'Or configure correct coin pool API address',
    orDisableCoinPoolOptions:
      'Or disable "Use Coin Pool" and "Use OI Top" options',
    signalSourceNotConfigured: 'Signal Source Not Configured',
    signalSourceWarningMessage:
      'You have traders that enabled "Use Coin Pool" or "Use OI Top", but signal source API address is not configured yet. This will cause candidate coins count to be 0, and traders cannot work properly.',
    configureSignalSourceNow: 'Configure Signal Source Now',

    // FAQ Page
    faqTitle: 'Frequently Asked Questions',
    faqSubtitle: 'Find answers to common questions about AUAIEX',
    faqStillHaveQuestions: 'Still Have Questions?',
    faqContactUs: 'Join our community or check our GitHub for more help',

    // FAQ Categories
    faqCategoryBasics: 'General Questions',
    faqCategoryContributing: 'Contributing & Tasks',
    faqCategorySetup: 'Setup & Configuration',
    faqCategoryTrading: 'Trading Questions',
    faqCategoryTechnical: 'Technical Issues',
    faqCategoryAI: 'AI & Model Questions',
    faqCategoryData: 'Data & Privacy',

    // FAQ Questions & Answers - General
    faqWhatIsAUAIEX: 'What is AUAIEX?',
    faqWhatIsAUAIEXAnswer:
      'AUAIEX is an AI-powered cryptocurrency trading bot that uses large language models (LLMs) to make trading decisions on futures markets.',

    faqSupportedExchanges: 'Which exchanges are supported?',
    faqSupportedExchangesAnswer:
      'Binance Futures and OKX Futures are supported.',

    faqIsProfitable: 'Is AUAIEX profitable?',
    faqIsProfitableAnswer:
      'AI trading is experimental and not guaranteed to be profitable. Always start with small amounts and never invest more than you can afford to lose.',

    faqMultipleTraders: 'Can I run multiple traders simultaneously?',
    faqMultipleTradersAnswer:
      'Yes! AUAIEX supports running multiple traders with different configurations, AI models, and trading strategies.',

    // Contributing & Community
    faqGithubProjectsTasks: 'How to use GitHub Projects and pick up tasks?',
    faqGithubProjectsTasksAnswer:
      'Roadmap: https://github.com/orgs/AuaiexAI/projects/3  • Task Dashboard: https://github.com/orgs/AuaiexAI/projects/5  • Steps: Open links → filter by labels (good first issue / help wanted / frontend / backend) → read Description & Acceptance Criteria → comment "assign me" or self-assign → Fork the repo → sync your fork\'s dev with upstream/dev → create a feature branch from your fork\'s dev → push to your fork → open PR (base: AuaiexAI/auaiex:dev ← compare: your-username/auaiex:feat/your-topic) → reference Issue (Closes #123) and use the proper template.',

    faqContributePR: 'How to properly submit PRs and contribute?',
    faqContributePRAnswer:
      "Guidelines: • Fork first; branch from your fork's dev (avoid direct commits to upstream main) • Branch naming: feat/..., fix/..., docs/...; Conventional Commits • Run checks before PR: npm --prefix web run lint && npm --prefix web run build • For UI changes, attach screenshots or a short video • Choose the proper PR template (frontend/backend/docs/general) • Open PR from your fork to AuaiexAI/auaiex:dev and link Issue (Closes #123) • Keep rebasing onto upstream/dev; ensure CI passes; prefer small, focused PRs • Read CONTRIBUTING.md and .github/PR_TITLE_GUIDE.md",

    // Setup & Configuration
    faqSystemRequirements: 'What are the system requirements?',
    faqSystemRequirementsAnswer:
      'OS: Linux, macOS, or Windows (Docker recommended); RAM: 2GB minimum, 4GB recommended; Disk: 1GB for application + logs; Network: Stable internet connection.',

    faqNeedCoding: 'Do I need coding experience?',
    faqNeedCodingAnswer:
      'No! AUAIEX has a web UI for all configuration. However, basic command line knowledge helps with setup and troubleshooting.',

    faqGetApiKeys: 'How do I get API keys?',
    faqGetApiKeysAnswer:
      'For Binance: Account → API Management → Create API → Enable Futures. For OKX: create an API key with futures trading permissions and keep the passphrase.',

    faqUseSubaccount: 'Should I use a subaccount?',
    faqUseSubaccountAnswer:
      'Recommended: Yes, use a subaccount dedicated to AUAIEX for better risk isolation. However, note that some subaccounts have restrictions (e.g., 5x max leverage on Binance).',

    faqDockerDeployment: 'Docker deployment keeps failing',
    faqDockerDeploymentAnswer:
      'Common issues: Network connection problems, dependency installation failures, insufficient memory (needs at least 2C2G). If stuck at "go build", try: docker compose down && docker compose build --no-cache && docker compose up -d',

    faqBalanceZero: 'Account balance shows 0',
    faqBalanceZeroAnswer:
      'Funds are likely in spot account instead of futures account, or locked in savings products. You need to manually transfer funds to futures account in Binance.',

    faqTestnet: 'Can I use testnet for testing?',
    faqTestnetAnswer:
      'Testnet is not supported at the moment. We recommend using real trading with small amounts (10-50 USDT) for testing.',

    // Trading Questions
    faqNoTrades: "Why isn't my trader making any trades?",
    faqNoTradesAnswer:
      'Common reasons: AI decided to "wait" due to market conditions; Insufficient balance or margin; Position limits reached (default: max 3 positions); Check troubleshooting guide for detailed diagnostics.',

    faqDecisionFrequency: 'How often does the AI make decisions?',
    faqDecisionFrequencyAnswer:
      'Configurable! Default is every 3-5 minutes. Too frequent = overtrading, too slow = missed opportunities.',

    faqCustomStrategy: 'Can I customize the trading strategy?',
    faqCustomStrategyAnswer:
      'Yes! You can adjust leverage settings, modify coin selection pool, change decision intervals, and customize system prompts (advanced).',

    faqMaxPositions: "What's the maximum number of concurrent positions?",
    faqMaxPositionsAnswer:
      'Default: 3 positions. This is a soft limit defined in the AI prompt, not hard-coded.',

    faqMarginInsufficient: 'Margin is insufficient error (code=-2019)',
    faqMarginInsufficientAnswer:
      'Common causes: Funds not transferred to futures account; Leverage set too high (default 20-50x); Existing positions using margin; Need to transfer USDT from spot to futures account first.',

    faqHighFees: 'Trading fees are too high',
    faqHighFeesAnswer:
      'AUAIEX default 3-minute scan interval can cause frequent trading. Solutions: Increase decision interval to 5-10 minutes; Optimize system prompt to reduce overtrading; Adjust leverage to reduce position sizes.',

    faqNoTakeProfit: "AI doesn't close profitable positions",
    faqNoTakeProfitAnswer:
      'AI may believe the trend will continue. The system lacks trailing stop-loss feature currently. You can manually close positions or adjust the system prompt to be more conservative with profit-taking.',

    // Technical Issues
    faqBinanceApiFailed: 'Binance API call failed (code=-2015)',
    faqBinanceApiFailedAnswer:
      'Error: "Invalid API-key, IP, or permissions for action". Solutions: Add server IP to Binance API whitelist; Check API permissions (needs Read + Futures Trading); Ensure using futures API not unified account API; VPN IP might be unstable.',

    faqBinancePositionMode: 'Binance Position Mode Error (code=-4061)',
    faqBinancePositionModeAnswer:
      'Error: "Order\'s position side does not match user\'s setting". Solution: Switch to Hedge Mode (双向持仓) in Binance Futures settings. You must close all positions first before switching.',

    faqPortInUse: "Backend won't start / Port already in use",
    faqPortInUseAnswer:
      'Check what\'s using port 8080 with "lsof -i :8080" and change the port in your .env file with AUAIEX_BACKEND_PORT=8081.',

    faqFrontendLoading: 'Frontend shows "Loading..." forever',
    faqFrontendLoadingAnswer:
      'Check if backend is running with "curl http://localhost:8080/api/health". Should return {"status":"ok"}. If not, check the troubleshooting guide.',

    faqDatabaseLocked: 'Database locked error',
    faqDatabaseLockedAnswer:
      'Stop all AUAIEX processes with "docker compose down" or "pkill auaiex", then restart with "docker compose up -d".',

    faqAiLearningFailed: 'AI learning data failed to load',
    faqAiLearningFailedAnswer:
      'Causes: TA-Lib library not properly installed; Insufficient historical data (need completed trades); Environment configuration issues. Install TA-Lib: pip install TA-Lib or check system dependencies.',

    faqConfigNotEffective: 'Configuration changes not taking effect',
    faqConfigNotEffectiveAnswer:
      'For Docker: Need to rebuild with "docker compose down && docker compose up -d --build". For PM2: Restart with "pm2 restart all". Check configuration file format and path are correct.',

    // AI & Model Questions
    faqWhichModels: 'Which AI models are supported?',
    faqWhichModelsAnswer:
      'DeepSeek (recommended for cost/performance), Qwen (Alibaba Cloud), and Custom OpenAI-compatible APIs (can be used for OpenAI, Claude via proxy, or other providers).',

    faqApiCosts: 'How much do API calls cost?',
    faqApiCostsAnswer:
      'Depends on your model and decision frequency: DeepSeek: ~$0.10-0.50 per day (1 trader, 5min intervals); Qwen: ~$0.20-0.80 per day; Custom API (e.g., OpenAI GPT-4): ~$2-5 per day. Estimates based on typical usage.',

    faqMultipleModels: 'Can I use multiple AI models?',
    faqMultipleModelsAnswer:
      'Yes! Each trader can use a different AI model. You can even A/B test different models.',

    faqAiLearning: 'Does the AI learn from its mistakes?',
    faqAiLearningAnswer:
      'Yes, to some extent. AUAIEX provides historical performance feedback in each decision prompt, allowing the AI to adjust its strategy.',

    faqOnlyShort: 'AI only opens short positions, no long positions',
    faqOnlyShortAnswer:
      'The default system prompt contains "Don\'t have a long bias! Shorting is one of your core tools" which may cause this. Also affected by 4-hour timeframe data and model training bias. You can modify the system prompt to be more balanced.',

    faqModelSelection: 'Which DeepSeek version should I use?',
    faqModelSelectionAnswer:
      "DeepSeek V3 is recommended for best performance. Alternatives: DeepSeek R1 (reasoning model, slower but better logic), SiliconFlow's DeepSeek (alternative API provider). Most users report good results with V3.",

    // Data & Privacy
    faqDataStorage: 'Where is my data stored?',
    faqDataStorageAnswer:
      'All data is stored locally on your machine in SQLite databases: data.db (all configurations and trade history), and decision_logs/ (AI decision records).',

    faqApiKeySecurity: 'Is my API key secure?',
    faqApiKeySecurityAnswer:
      'API keys are stored in local databases. Never share your databases or .env files. We recommend using API keys with IP whitelist restrictions.',

    faqExportHistory: 'Can I export my trading history?',
    faqExportHistoryAnswer:
      'Yes! Trading data is in SQLite format. You can query it directly with: sqlite3 trading.db "SELECT * FROM trades;"',

    faqGetHelp: 'Where can I get help?',
    faqGetHelpAnswer:
      'Check GitHub Discussions, join our Telegram Community, or open an issue on GitHub.',

    // Web Crypto Environment Check
    environmentCheck: {
      button: 'Check Secure Environment',
      checking: 'Checking...',
      description:
        'Automatically verifying whether this browser context allows Web Crypto before entering sensitive keys.',
      secureTitle: 'Secure context detected',
      secureDesc:
        'Web Crypto API is available. You can continue entering secrets with encryption enabled.',
      insecureTitle: 'Insecure context detected',
      insecureDesc:
        'This page is not running over HTTPS or a trusted localhost origin, so browsers block Web Crypto calls.',
      tipsTitle: 'How to fix:',
      tipHTTPS:
        'Serve the dashboard over HTTPS with a valid certificate (IP origins also need TLS).',
      tipLocalhost:
        'During development, open the app via http://localhost or 127.0.0.1.',
      tipIframe:
        'Avoid embedding the app in insecure HTTP iframes or reverse proxies that strip HTTPS.',
      unsupportedTitle: 'Browser does not expose Web Crypto',
      unsupportedDesc:
        'Open AUAIEX over HTTPS (or http://localhost during development) and avoid insecure iframes/reverse proxies so the browser can enable Web Crypto.',
      summary: 'Current origin: {origin} • Protocol: {protocol}',
      disabledTitle: 'Transport encryption disabled',
      disabledDesc:
        'Server-side transport encryption is disabled. API keys will be transmitted in plaintext. Enable TRANSPORT_ENCRYPTION=true for enhanced security.',
    },

    environmentSteps: {
      checkTitle: '1. Environment check',
      selectTitle: '2. Select exchange',
    },

    // Two-Stage Key Modal
    twoStageKey: {
      title: 'Two-Stage Private Key Input',
      stage1Description:
        'Enter the first {length} characters of your private key',
      stage2Description:
        'Enter the remaining {length} characters of your private key',
      stage1InputLabel: 'First Part',
      stage2InputLabel: 'Second Part',
      characters: 'characters',
      processing: 'Processing...',
      nextButton: 'Next',
      cancelButton: 'Cancel',
      backButton: 'Back',
      encryptButton: 'Encrypt & Submit',
      obfuscationCopied: 'Obfuscation data copied to clipboard',
      obfuscationInstruction:
        'Paste something else to clear clipboard, then continue',
      obfuscationManual: 'Manual obfuscation required',
    },

    // Error Messages
    errors: {
      privatekeyIncomplete: 'Please enter at least {expected} characters',
      privatekeyInvalidFormat:
        'Invalid private key format (should be 64 hex characters)',
      privatekeyObfuscationFailed: 'Clipboard obfuscation failed',
    },
  },
  zh: {
    // Header
    appTitle: 'AUAIEX',
    subtitle: '多AI模型交易平台',
    aiTraders: 'AI交易员',
    details: '详情',
    tradingPanel: '交易面板',
    competition: '竞赛',
    backtest: '回测',
    running: '运行中',
    stopped: '已停止',
    adminMode: '管理员模式',
    logout: '退出',
    switchTrader: '切换交易员:',
    view: '查看',

    // Navigation
    realtimeNav: '实时',
    configNav: '配置',
    dashboardNav: '看板',
    strategyNav: '策略',
    faqNav: '常见问题',

    // Footer
    footerTitle: 'AUAIEX - AI交易系统',
    footerWarning: '⚠️ 交易有风险，请谨慎使用。',

    // Stats Cards
    totalEquity: '总净值',
    availableBalance: '可用余额',
    totalPnL: '总盈亏',
    positions: '持仓',
    margin: '保证金',
    free: '空闲',

    // Positions Table
    currentPositions: '当前持仓',
    active: '活跃',
    symbol: '币种',
    side: '方向',
    entryPrice: '入场价',
    markPrice: '标记价',
    quantity: '数量',
    positionValue: '仓位价值',
    leverage: '杠杆',
    unrealizedPnL: '未实现盈亏',
    liqPrice: '强平价',
    long: '多头',
    short: '空头',
    noPositions: '无持仓',
    noActivePositions: '当前没有活跃的交易持仓',
    actionCol: '操作',
    entryShort: '入场价',
    markShort: '标记价',
    qtyShort: '数量',
    valueShort: '价值',
    levShort: '杠杆',
    upnlShort: '未实现盈亏',
    liqShort: '强平价',
    closeBtn: '平仓',
    closePositionTitle: '平仓',
    confirmCloseTitle: '确认平仓',
    confirmBtn: '确认',
    cancelBtn: '取消',
    closeSuccess: '平仓成功',
    closeFailed: '平仓失败',
    confirmClosePosition: '确定要平仓 {symbol} {side}吗？',
    longPosition: '多仓',
    shortPosition: '空仓',
    liveExposure: '实时敞口',

    // Order & Fill History
    executionHistory: '执行历史',
    orderHistory: '历史订单',
    ordersTab: '订单',
    fillsTab: '成交',
    recordsUnit: '条',
    timeCol: '时间',
    priceCol: '价格',
    feeCol: '手续费',
    statusCol: '状态',
    realizedPnL: '已实现盈亏',
    noOrdersYet: '暂无订单',
    noOrdersDesc: '交易员执行交易后，订单记录将显示在这里',
    noFillsYet: '暂无成交',
    noFillsDesc: '交易完成后，交易所成交记录将显示在这里',
    protectionActive: '止损/止盈保护已生效',

    // Performance Metrics
    riskAnalytics: '风险与分析',
    performanceMetrics: '绩效指标',
    closedWinRate: '平仓胜率',
    profitFactor: '盈亏比',
    grossProfit: '总盈利',
    grossLoss: '总亏损',
    avgWinLabel: '平均盈利',
    avgLossLabel: '平均亏损',
    pnlBySymbol: '按币种已实现盈亏',

    // Export
    exportCsv: '导出 CSV',
    exportPositions: '将当前持仓导出为 CSV',
    exportDecisions: '将决策日志（含思维链）导出为 JSON',

    // Notifications page
    notificationsNav: '通知',
    notificationsTitle: '通知管理',
    notificationsDesc:
      '将交易告警（开仓/平仓/失败）推送到 Telegram、Webhook 或邮箱渠道',
    addChannel: '添加渠道',
    editChannel: '编辑渠道',
    telegramChannel: 'Telegram',
    webhookChannel: 'Webhook',
    emailChannel: '邮箱',
    channelName: '渠道名称',
    channelType: '渠道类型',
    channelNameRequired: '请填写渠道名称',
    channelSaved: '渠道已保存',
    channelDeleted: '渠道已删除',
    sendTest: '发送测试',
    testSent: '测试通知已发送',
    testFailed: '测试通知发送失败',
    noChannels: '暂无通知渠道',
    noChannelsDesc: '添加 Telegram / Webhook / 邮箱渠道以接收交易告警',
    deliveryLogs: '发送日志',
    noDeliveryLogs: '暂无发送日志',
    eventCol: '事件',
    messageCol: '内容',
    channelNameCol: '渠道',
    failedCol: '失败',
    secretField: '密钥',
    channelHelp:
      'Telegram：通过 @BotFather 创建机器人，先给机器人发一条消息（或将其拉入群），再填入 chat_id。Webhook 接收 JSON POST，设置 secret 后会附带 HMAC-SHA256 签名头 X-Auaiex-Signature。邮箱使用你自己的 SMTP 服务器。',

    // Settings page
    settingsNav: '设置',
    settingsTitle: '账户设置',
    settingsDesc: '管理个人资料、密码与双因素认证',
    accountInfo: '账户信息',
    emailCol: '邮箱',
    registeredAt: '注册时间',
    changePassword: '修改密码',
    oldPassword: '当前密码',
    newPasswordPlaceholder8: '新密码（至少 8 位）',
    confirmNewPassword: '确认新密码',
    updatePassword: '更新密码',
    newPasswordTooShort: '新密码至少需要 8 个字符',
    settingsPasswordMismatch: '两次输入的新密码不一致',
    passwordChanged: '密码修改成功',
    twofaManagement: '双因素认证（2FA）',
    twofaDesc:
      '重置会生成新的 TOTP 密钥，需用验证器 App 重新扫码并输入 6 位验证码确认后才会重新启用 2FA。启用后登录需要验证码。',
    resetTwofa: '重置 2FA 密钥',
    twofaResetDone: '新密钥已生成，请扫码并确认',
    twofaScanHint:
      '使用验证器 App（如 Google Authenticator）扫描二维码，然后输入 6 位验证码确认：',
    twofaEnabled: '2FA 已启用',
    verificationCode: '6 位验证码',

    // Admin page
    adminNav: '管理',
    adminTitle: '管理后台',
    adminDesc: '系统健康、用户概览与全局交易动态（仅 admin 账户可用）',
    adminUsersTotal: '用户数',
    adminVerified: '已验证',
    adminTradersTotal: '交易员',
    adminTradersRunning: '运行中',
    adminMemory: '内存',
    adminGoroutines: '协程',
    adminUserList: '用户列表',
    adminTraderCount: '交易员数',
    adminRunningCount: '运行中',
    adminNoUsers: '暂无用户',
    adminRecentActivity: '最近动态（全部交易员）',

    // Manual order
    manualOrder: '手动下单',
    confirmManualOrder:
      '确认为 {symbol} 提交 {action} 订单？将执行与 AI 决策相同的风控校验和保护单。',
    manualOrderSuccess: '手动下单已执行',
    manualOrderFailed: '手动下单失败',
    stopLossPctLabel: '止损百分比（相对入场价）',
    takeProfitPctLabel: '止盈百分比（相对入场价）',
    submitOrder: '提交订单',
    manualOrderHelp:
      '仓位大小与杠杆由后端风控引擎计算（与 AI 交易一致）；止损/止盈百分比会按当前市场价格转换为绝对价格。',
    manualCloseHelp: '将按市价全部平掉该币种该方向的持仓。',

    // Batch trader management
    selectAll: '全选',
    batchStart: '批量启动',
    batchStop: '批量停止',

    // Theme
    switchToLight: '切换到亮色主题',
    switchToDark: '切换到暗色主题',

    // 风控操作
    closeAll: '一键全平',
    confirmCloseAll: '确定要平掉全部 {count} 个持仓吗？此操作不可撤销。',
    closeAllSuccess: '已全部平仓',
    closeAllPartial: '{success}/{total} 个持仓已平仓，{failed} 个失败',
    distanceToLiq: '距强平',
    liqWarningTitle: '强平风险',
    stopTrading: '停止交易',
    confirmStopTrading: '确定停止该交易员的自动交易吗？',
    traderStopped: '已停止交易',
    stopTradingFailed: '停止交易失败',

    // 交易统计
    tradingStats: '交易统计',
    cycleSuccessRate: '周期成功率',
    totalCyclesLabel: '总周期数',
    openTradesTotal: '累计开仓',
    closeTradesTotal: '累计平仓',
    maxDrawdown: '最大回撤',
    noStatsData: '暂无统计数据',

    // 浏览器通知
    enableNotifications: '开启通知',
    notificationsEnabled: '通知已开启',
    notificationDenied: '浏览器已拒绝通知权限',
    notifPositionOpened: '{trader} 开仓 {side} {symbol}',
    notifPositionClosed: '{trader} 已平仓 {side} {symbol}',
    notifLiqRisk: '{symbol} 距强平价仅 {pct}%',
    notifDecisionError: '{trader} 决策周期执行失败',

    // 连接状态
    connectionLost: '连接已断开 — 显示的是 {time} 的数据',
    dataStale: '数据可能已过期 — 最后更新 {time}',
    liveLabel: '实时',

    // 仪表盘头部
    autoTradingActive: '自动交易运行中',
    traderOverview: '交易员概览',
    cyclesCount: '{count} 次循环',
    updatedAt: '更新于 {time}',
    noStrategy: '无策略',

    // 最近决策
    recentDecisions: '最近决策',
    lastCycles: '最近 {count} 个交易周期',
    noDecisionsYet: '暂无决策',
    aiDecisionsWillAppear: 'AI交易决策将显示在这里',
    cycle: '周期',
    success: '成功',
    failed: '失败',
    inputPrompt: '输入提示',
    aiThinking: '💭 AI思维链分析',
    collapse: '▼ 收起',
    expand: '▶ 展开',

    // Equity Chart
    accountEquityCurve: '账户净值曲线',
    noHistoricalData: '暂无历史数据',
    dataWillAppear: '运行几个周期后将显示收益率曲线',
    initialBalance: '初始余额',
    currentEquity: '当前净值',
    historicalCycles: '历史周期',
    displayRange: '显示范围',
    recent: '最近',
    allData: '全部数据',
    cycles: '个',

    // Comparison Chart
    comparisonMode: '对比模式',
    dataPoints: '数据点数',
    currentGap: '当前差距',
    count: '{count} 个',

    // TradingView Chart
    marketChart: '行情图表',
    viewChart: '点击查看图表',
    enterSymbol: '输入币种...',
    popularSymbols: '热门币种',
    fullscreen: '全屏',
    exitFullscreen: '退出全屏',

    // Backtest Page
    backtestPage: {
      title: '回测实验室',
      subtitle: '选择模型与时间范围，快速复盘 AI 决策链路。',
      start: '启动回测',
      starting: '启动中...',
      quickRanges: {
        h24: '24小时',
        d3: '3天',
        d7: '7天',
      },
      actions: {
        pause: '暂停',
        resume: '恢复',
        stop: '停止',
      },
      states: {
        running: '运行中',
        paused: '已暂停',
        completed: '已完成',
        failed: '失败',
        liquidated: '已爆仓',
      },
      form: {
        aiModelLabel: 'AI 模型',
        selectAiModel: '选择AI模型',
        providerLabel: 'Provider',
        statusLabel: '状态',
        enabled: '已启用',
        disabled: '未启用',
        noModelWarning: '请先在「模型配置」页面添加并启用AI模型。',
        runIdLabel: 'Run ID',
        runIdPlaceholder: '留空则自动生成',
        decisionTfLabel: '决策周期',
        cadenceLabel: '决策节奏（根数）',
        timeRangeLabel: '时间范围',
        symbolsLabel: '交易标的（逗号分隔）',
        customTfPlaceholder: '自定义周期（逗号分隔，例如 2h,6h）',
        initialBalanceLabel: '初始资金 (USDT)',
        feeLabel: '手续费 (bps)',
        slippageLabel: '滑点 (bps)',
        btcEthLeverageLabel: 'BTC/ETH 杠杆 (倍)',
        altcoinLeverageLabel: '山寨币杠杆 (倍)',
        fillPolicies: {
          nextOpen: '下一根开盘价',
          barVwap: 'K线 VWAP',
          midPrice: '中间价',
        },
        promptPresets: {
          baseline: '基础版',
          aggressive: '激进版',
          conservative: '稳健版',
          scalping: '剥头皮',
        },
        cacheAiLabel: '复用AI缓存',
        replayOnlyLabel: '仅回放记录',
        overridePromptLabel: '仅使用自定义提示词',
        customPromptLabel: '自定义提示词（可选）',
        customPromptPlaceholder: '追加或完全自定义策略提示词',
      },
      runList: {
        title: '运行列表',
        count: '共 {count} 条记录',
      },
      compare: {
        title: '回测对比',
        desc: '选择最多 {max} 个已完成的回测，叠加归一化收益率曲线并对比核心指标（最优值高亮）',
        clear: '清除选择',
        noFinished: '暂无可对比的已完成回测',
        metric: '指标',
        totalReturn: '总收益率',
        maxDrawdown: '最大回撤',
        sharpe: '夏普比率',
        profitFactor: '盈亏比',
        winRate: '胜率',
        trades: '交易次数',
      },
      filters: {
        allStates: '全部状态',
        searchPlaceholder: 'Run ID / 标签',
      },
      tableHeaders: {
        runId: 'Run ID',
        label: '标签',
        state: '状态',
        progress: '进度',
        equity: '净值',
        lastError: '最后错误',
        updated: '更新时间',
      },
      emptyStates: {
        noRuns: '暂无记录',
        selectRun: '请选择一个运行查看详情',
      },
      detail: {
        tfAndSymbols: '周期: {tf} · 币种 {count}',
        labelPlaceholder: '备注标签',
        saveLabel: '保存',
        deleteLabel: '删除',
        exportLabel: '导出',
        errorLabel: '错误',
      },
      toasts: {
        selectModel: '请先选择一个AI模型。',
        modelDisabled: 'AI模型 {name} 尚未启用。',
        invalidRange: '结束时间必须晚于开始时间。',
        startSuccess: '回测 {id} 已启动。',
        startFailed: '启动失败，请稍后再试。',
        actionSuccess: '{action} {id} 成功。',
        actionFailed: '操作失败，请稍后再试。',
        labelSaved: '标签已更新。',
        labelFailed: '更新标签失败。',
        confirmDelete: '确认删除回测 {id} 吗？该操作不可恢复。',
        deleteSuccess: '回测记录已删除。',
        deleteFailed: '删除失败，请稍后再试。',
        traceFailed: '获取AI思维链失败。',
        exportSuccess: '已导出 {id} 的数据。',
        exportFailed: '导出失败。',
      },
      aiTrace: {
        title: 'AI 思维链',
        clear: '清除',
        cyclePlaceholder: '循环编号',
        fetch: '获取',
        prompt: '提示词',
        cot: '思考链',
        output: '输出',
        cycleTag: '周期 #{cycle}',
      },
      decisionTrail: {
        title: 'AI 决策轨迹',
        subtitle: '展示最近 {count} 次循环',
        empty: '暂无记录',
        emptyHint: '回测运行后将自动记录每次 AI 思考与执行',
      },
      charts: {
        equityTitle: '净值曲线',
        equityEmpty: '暂无数据',
      },
      metrics: {
        title: '指标',
        totalReturn: '总收益率 %',
        maxDrawdown: '最大回撤 %',
        sharpe: '夏普比率',
        profitFactor: '盈亏因子',
        pending: '计算中...',
        realized: '已实现盈亏',
        unrealized: '未实现盈亏',
      },
      trades: {
        title: '交易事件',
        headers: {
          time: '时间',
          symbol: '币种',
          action: '操作',
          qty: '数量',
          leverage: '杠杆',
          pnl: '盈亏',
        },
        empty: '暂无交易',
      },
      metadata: {
        title: '元信息',
        created: '创建时间',
        updated: '更新时间',
        processedBars: '已处理K线',
        maxDrawdown: '最大回撤',
        liquidated: '是否爆仓',
        yes: '是',
        no: '否',
      },
    },

    // Competition Page
    aiCompetition: 'AI竞赛',
    traders: '交易员',
    liveBattle: '实时对战',
    realTimeBattle: '实时对战',
    leader: '领先者',
    leaderboard: '排行榜',
    live: '实时',
    realTime: '实时',
    performanceComparison: '表现对比',
    realTimePnL: '实时收益率',
    realTimePnLPercent: '实时收益率',
    headToHead: '正面对决',
    leadingBy: '领先 {gap}%',
    behindBy: '落后 {gap}%',
    equity: '权益',
    pnl: '收益',
    pos: '持仓',

    // AI Traders Management
    manageAITraders: '管理您的AI交易机器人',
    aiModels: 'AI模型',
    exchanges: '交易所',
    createTrader: '创建交易员',
    modelConfiguration: '模型配置',
    configured: '已配置',
    notConfigured: '未配置',
    currentTraders: '当前交易员',
    noTraders: '暂无AI交易员',
    createFirstTrader: '创建您的第一个AI交易员开始使用',
    dashboardEmptyTitle: '开始使用吧！',
    dashboardEmptyDescription:
      '创建您的第一个 AI 交易员，自动化您的交易策略。连接交易所、选择 AI 模型，几分钟内即可开始交易！',
    goToTradersPage: '创建您的第一个交易员',
    configureModelsFirst: '请先配置AI模型',
    configureExchangesFirst: '请先配置交易所',
    configureModelsAndExchangesFirst: '请先配置AI模型和交易所',
    modelNotConfigured: '所选模型未配置',
    exchangeNotConfigured: '所选交易所未配置',
    confirmDeleteTrader: '确定要删除这个交易员吗？',
    status: '状态',
    start: '启动',
    stop: '停止',
    createNewTrader: '创建新的AI交易员',
    selectAIModel: '选择AI模型',
    selectExchange: '选择交易所',
    traderName: '交易员名称',
    enterTraderName: '输入交易员名称',
    cancel: '取消',
    create: '创建',
    configureAIModels: '配置AI模型',
    configureExchanges: '配置交易所',
    aiScanInterval: 'AI 扫描决策间隔 (分钟)',
    scanIntervalRecommend: '建议: 3-10分钟',
    useTestnet: '使用测试网',
    enabled: '启用',
    save: '保存',

    // AI Model Configuration
    officialAPI: '官方API',
    customAPI: '自定义API',
    apiKey: 'API密钥',
    customAPIURL: '自定义API地址',
    enterAPIKey: '请输入API密钥',
    enterCustomAPIURL: '请输入自定义API端点地址',
    useOfficialAPI: '使用官方API服务',
    useCustomAPI: '使用自定义API端点',

    // Exchange Configuration
    secretKey: '密钥',
    privateKey: '私钥',
    walletAddress: '钱包地址',
    user: '用户名',
    signer: '签名者',
    passphrase: '口令',
    enterSecretKey: '输入密钥',
    enterPrivateKey: '输入私钥',
    enterWalletAddress: '输入钱包地址',
    enterUser: '输入用户名',
    enterSigner: '输入签名者地址',
    enterPassphrase: '输入Passphrase',
    hyperliquidPrivateKeyDesc: 'Hyperliquid 使用私钥进行交易认证',
    hyperliquidWalletAddressDesc: '与私钥对应的钱包地址',
    // Hyperliquid 代理钱包 (新安全模型)
    hyperliquidAgentWalletTitle: 'Hyperliquid 代理钱包配置',
    hyperliquidAgentWalletDesc:
      '使用代理钱包安全交易：代理钱包用于签名（餘額~0），主钱包持有资金（永不暴露私钥）',
    hyperliquidAgentPrivateKey: '代理私钥',
    enterHyperliquidAgentPrivateKey: '输入代理钱包私钥',
    hyperliquidAgentPrivateKeyDesc: '代理钱包仅有交易权限，无法提现',
    hyperliquidMainWalletAddress: '主钱包地址',
    enterHyperliquidMainWalletAddress: '输入主钱包地址',
    hyperliquidMainWalletAddressDesc:
      '持有交易资金的主钱包地址（永不暴露其私钥）',
    // Aster API Pro 配置
    asterApiProTitle: 'Aster API Pro 代理钱包配置',
    asterApiProDesc:
      '使用 API Pro 代理钱包安全交易：代理钱包用于签名交易，主钱包持有资金（永不暴露主钱包私钥）',
    asterUserDesc:
      '主钱包地址 - 您用于登录 Aster 的 EVM 钱包地址（仅支持 EVM 钱包）',
    asterSignerDesc:
      'API Pro 代理钱包地址 (0x...) - 从 https://www.asterdex.com/zh-CN/api-wallet 生成',
    asterPrivateKeyDesc:
      'API Pro 代理钱包私钥 - 从 https://www.asterdex.com/zh-CN/api-wallet 获取（仅在本地用于签名，不会被传输）',
    asterUsdtWarning:
      '重要提示：Aster 仅统计 USDT 余额。请确保您使用 USDT 作为保证金币种，避免其他资产（BNB、ETH等）的价格波动导致盈亏统计错误',
    asterUserLabel: '主钱包地址',
    asterSignerLabel: 'API Pro 代理钱包地址',
    asterPrivateKeyLabel: 'API Pro 代理钱包私钥',
    enterAsterUser: '输入主钱包地址 (0x...)',
    enterAsterSigner: '输入 API Pro 代理钱包地址 (0x...)',
    enterAsterPrivateKey: '输入 API Pro 代理钱包私钥',

    // LIGHTER 配置
    lighterWalletAddress: 'L1 錢包地址',
    lighterPrivateKey: 'L1 私鑰',
    lighterApiKeyPrivateKey: 'API Key 私鑰',
    enterLighterWalletAddress: '請輸入以太坊錢包地址（0x...）',
    enterLighterPrivateKey: '請輸入 L1 私鑰（32 字節）',
    enterLighterApiKeyPrivateKey: '請輸入 API Key 私鑰（40 字節，可選）',
    lighterWalletAddressDesc: '您的以太坊錢包地址，用於識別賬戶',
    lighterPrivateKeyDesc: 'L1 私鑰用於賬戶識別（32 字節 ECDSA 私鑰）',
    lighterApiKeyPrivateKeyDesc:
      'API Key 私鑰用於簽名交易（40 字節 Poseidon2 私鑰）',
    lighterApiKeyOptionalNote:
      '如果不提供 API Key，系統將使用功能受限的 V1 模式',
    lighterV1Description: '基本模式 - 功能受限，僅用於測試框架',
    lighterV2Description: '完整模式 - 支持 Poseidon2 簽名和真實交易',
    lighterPrivateKeyImported: 'LIGHTER 私鑰已導入',

    // Exchange names
    hyperliquidExchangeName: 'Hyperliquid',
    asterExchangeName: 'Aster DEX',

    // Secure input
    secureInputButton: '安全输入',
    secureInputReenter: '重新安全输入',
    secureInputClear: '清除',
    secureInputHint:
      '已通过安全双阶段输入设置。若需修改，请点击"重新安全输入"。',

    // Two Stage Key Modal
    twoStageModalTitle: '安全私钥输入',
    twoStageModalDescription: '使用双阶段流程安全输入长度为 {length} 的私钥。',
    twoStageStage1Title: '步骤一 · 输入前半段',
    twoStageStage1Placeholder: '前 32 位字符（若有 0x 前缀请保留）',
    twoStageStage1Hint:
      '继续后会将扰动字符串复制到剪贴板，用于迷惑剪贴板监控。',
    twoStageStage1Error: '请先输入第一段私钥。',
    twoStageNext: '下一步',
    twoStageProcessing: '处理中…',
    twoStageCancel: '取消',
    twoStageStage2Title: '步骤二 · 输入剩余部分',
    twoStageStage2Placeholder: '剩余的私钥字符',
    twoStageStage2Hint: '将扰动字符串粘贴到任意位置后，再完成私钥输入。',
    twoStageClipboardSuccess:
      '扰动字符串已复制。请在完成前在任意文本处粘贴一次以迷惑剪贴板记录。',
    twoStageClipboardReminder:
      '记得在提交前粘贴一次扰动字符串，降低剪贴板泄漏风险。',
    twoStageClipboardManual: '自动复制失败，请手动复制下面的扰动字符串。',
    twoStageBack: '返回',
    twoStageSubmit: '确认',
    twoStageInvalidFormat:
      '私钥格式不正确，应为 {length} 位十六进制字符（可选 0x 前缀）。',
    testnetDescription: '启用后将连接到交易所测试环境,用于模拟交易',
    securityWarning: '安全提示',
    saveConfiguration: '保存配置',

    // Trader Configuration
    positionMode: '仓位模式',
    crossMarginMode: '全仓模式',
    isolatedMarginMode: '逐仓模式',
    crossMarginDescription: '全仓模式：所有仓位共享账户余额作为保证金',
    isolatedMarginDescription: '逐仓模式：每个仓位独立管理保证金，风险隔离',
    leverageConfiguration: '杠杆配置',
    btcEthLeverage: 'BTC/ETH杠杆',
    altcoinLeverage: '山寨币杠杆',
    leverageRecommendation: '推荐：BTC/ETH 5-10倍，山寨币 3-5倍，控制风险',
    tradingSymbols: '交易币种',
    tradingSymbolsPlaceholder:
      '输入币种，逗号分隔（如：BTCUSDT,ETHUSDT,SOLUSDT）',
    selectSymbols: '选择币种',
    selectTradingSymbols: '选择交易币种',
    selectedSymbolsCount: '已选择 {count} 个币种',
    clearSelection: '清空选择',
    confirmSelection: '确认选择',
    tradingSymbolsDescription:
      '留空 = 使用默认币种。必须以USDT结尾（如：BTCUSDT, ETHUSDT）',
    btcEthLeverageValidation: 'BTC/ETH杠杆必须在1-50倍之间',
    altcoinLeverageValidation: '山寨币杠杆必须在1-20倍之间',
    invalidSymbolFormat: '无效的币种格式：{symbol}，必须以USDT结尾',

    // System Prompt Templates
    systemPromptTemplate: '系统提示词模板',
    promptTemplateDefault: '默认稳健',
    promptTemplateAdaptive: '保守策略',
    promptTemplateAdaptiveRelaxed: '激进策略',
    promptTemplateHansen: 'Hansen 策略',
    promptTemplateNof1: 'NoF1 英文框架',
    promptTemplateTaroLong: 'Taro 长仓',
    promptDescDefault: '📊 默认稳健策略',
    promptDescDefaultContent:
      '最大化夏普比率，平衡风险收益，适合新手和长期稳定交易',
    promptDescAdaptive: '🛡️ 保守策略 (v6.0.0)',
    promptDescAdaptiveContent:
      '严格风控，BTC 强制确认，高胜率优先，适合保守型交易者',
    promptDescAdaptiveRelaxed: '⚡ 激进策略 (v6.0.0)',
    promptDescAdaptiveRelaxedContent:
      '高频交易，BTC 可选确认，追求交易机会，适合波动市场',
    promptDescHansen: '🎯 Hansen 策略',
    promptDescHansenContent: 'Hansen 定制策略，最大化夏普比率，专业交易者专用',
    promptDescNof1: '🌐 NoF1 英文框架',
    promptDescNof1Content:
      'Hyperliquid 交易所专用，英文提示词，风险调整回报最大化',
    promptDescTaroLong: '📈 Taro 长仓策略',
    promptDescTaroLongContent:
      '数据驱动决策，多维度验证，持续学习进化，长仓专用',

    // Loading & Error
    loading: '加载中...',

    // AI Traders Page - Additional
    inUse: '正在使用',
    noModelsConfigured: '暂无已配置的AI模型',
    noExchangesConfigured: '暂无已配置的交易所',
    signalSource: '信号源',
    signalSourceConfig: '信号源配置',
    coinPoolDescription: '用于获取币种池数据的API地址，留空则不使用此信号源',
    oiTopDescription: '用于获取持仓量排行数据的API地址，留空则不使用此信号源',
    information: '说明',
    signalSourceInfo1:
      '• 信号源配置为用户级别，每个用户可以设置自己的信号源URL',
    signalSourceInfo2: '• 在创建交易员时可以选择是否使用这些信号源',
    signalSourceInfo3: '• 配置的URL将用于获取市场数据和交易信号',
    editAIModel: '编辑AI模型',
    addAIModel: '添加AI模型',
    confirmDeleteModel: '确定要删除此AI模型配置吗？',
    cannotDeleteModelInUse: '无法删除此AI模型，因为有交易员正在使用',
    tradersUsing: '正在使用此配置的交易员',
    pleaseDeleteTradersFirst: '请先删除或重新配置这些交易员',
    selectModel: '选择AI模型',
    pleaseSelectModel: '请选择模型',
    customBaseURL: 'Base URL (可选)',
    customBaseURLPlaceholder: '自定义API基础URL，如: https://api.openai.com/v1',
    leaveBlankForDefault: '留空则使用默认API地址',
    modelConfigInfo1: '• 使用官方 API 时，只需填写 API Key，其他字段留空即可',
    modelConfigInfo2:
      '• 自定义 Base URL 和 Model Name 仅在使用第三方代理时需要填写',
    modelConfigInfo3: '• API Key 加密存储，不会明文展示',
    defaultModel: '默认模型',
    applyApiKey: '申请 API Key',
    kimiApiNote:
      'Kimi 需要从国际站申请 API Key (moonshot.ai)，中国区 Key 不通用',
    leaveBlankForDefaultModel: '留空使用默认模型名称',
    customModelName: 'Model Name (可选)',
    customModelNamePlaceholder: '例如: deepseek-chat, qwen3-max, gpt-4o',
    saveConfig: '保存配置',
    editExchange: '编辑交易所',
    addExchange: '添加交易所',
    confirmDeleteExchange: '确定要删除此交易所配置吗？',
    cannotDeleteExchangeInUse: '无法删除此交易所，因为有交易员正在使用',
    pleaseSelectExchange: '请选择交易所',
    exchangeConfigWarning1: '• API密钥将被加密存储，建议使用只读或期货交易权限',
    exchangeConfigWarning2: '• 不要授予提现权限，确保资金安全',
    exchangeConfigWarning3: '• 删除配置后，相关交易员将无法正常交易',
    edit: '编辑',
    viewGuide: '查看教程',
    binanceSetupGuide: '币安配置教程',
    closeGuide: '关闭',
    whitelistIP: '白名单IP',
    whitelistIPDesc: '币安交易所需要填写白名单IP',
    serverIPAddresses: '服务器IP地址',
    copyIP: '复制',
    ipCopied: 'IP已复制',
    copyIPFailed: 'IP地址复制失败，请手动复制',
    loadingServerIP: '正在加载服务器IP...',

    // Error Messages
    createTraderFailed: '创建交易员失败',
    getTraderConfigFailed: '获取交易员配置失败',
    modelConfigNotExist: 'AI模型配置不存在或未启用',
    exchangeConfigNotExist: '交易所配置不存在或未启用',
    updateTraderFailed: '更新交易员失败',
    deleteTraderFailed: '删除交易员失败',
    operationFailed: '操作失败',
    deleteConfigFailed: '删除配置失败',
    modelNotExist: '模型不存在',
    saveConfigFailed: '保存配置失败',
    exchangeNotExist: '交易所不存在',
    deleteExchangeConfigFailed: '删除交易所配置失败',
    saveSignalSourceFailed: '保存信号源配置失败',
    encryptionFailed: '加密敏感数据失败',

    // Login & Register
    login: '登录',
    register: '注册',
    username: '用户名',
    email: '邮箱',
    password: '密码',
    confirmPassword: '确认密码',
    usernamePlaceholder: '请输入用户名',
    emailPlaceholder: '请输入邮箱地址',
    passwordPlaceholder: '请输入密码（至少6位）',
    confirmPasswordPlaceholder: '请再次输入密码',
    passwordRequirements: '密码要求',
    passwordRuleMinLength: '至少 8 位',
    passwordRuleUppercase: '至少 1 个大写字母',
    passwordRuleLowercase: '至少 1 个小写字母',
    passwordRuleNumber: '至少 1 个数字',
    passwordRuleSpecial: '至少 1 个特殊字符（@#$%!&*?）',
    passwordRuleMatch: '两次密码一致',
    passwordNotMeetRequirements: '密码不符合安全要求',
    otpPlaceholder: '000000',
    loginTitle: '登录到您的账户',
    registerTitle: '创建新账户',
    loginButton: '登录',
    registerButton: '注册',
    back: '返回',
    noAccount: '还没有账户？',
    hasAccount: '已有账户？',
    registerNow: '立即注册',
    loginNow: '立即登录',
    forgotPassword: '忘记密码？',
    rememberMe: '记住我',
    resetPassword: '重置密码',
    resetPasswordTitle: '重置您的密码',
    newPassword: '新密码',
    newPasswordPlaceholder: '请输入新密码（至少6位）',
    resetPasswordButton: '重置密码',
    resetPasswordSuccess: '密码重置成功！请使用新密码登录',
    resetPasswordFailed: '密码重置失败',
    backToLogin: '返回登录',
    otpCode: 'OTP验证码',
    scanQRCode: '扫描二维码',
    enterOTPCode: '输入6位OTP验证码',
    verifyOTP: '验证OTP',
    setupTwoFactor: '设置双因素认证',
    setupTwoFactorDesc: '请按以下步骤设置Google验证器以保护您的账户安全',
    scanQRCodeInstructions: '使用Google Authenticator或Authy扫描此二维码',
    otpSecret: '或手动输入此密钥：',
    qrCodeHint: '二维码（如果无法扫描，请使用下方密钥）：',
    authStep1Title: '步骤1：下载Google Authenticator',
    authStep1Desc: '在手机应用商店下载并安装Google Authenticator应用',
    authStep2Title: '步骤2：添加账户',
    authStep2Desc: '在应用中点击“+”，选择“扫描二维码”或“手动输入密钥”',
    authStep3Title: '步骤3：验证设置',
    authStep3Desc: '设置完成后，点击下方按钮输入6位验证码',
    setupCompleteContinue: '我已完成设置，继续',
    copy: '复制',
    completeRegistration: '完成注册',
    completeRegistrationSubtitle: '以完成注册',
    loginSuccess: '登录成功',
    registrationSuccess: '注册成功',
    loginFailed: '登录失败，请检查您的邮箱和密码。',
    registrationFailed: '注册失败，请重试。',
    verificationFailed: 'OTP 验证失败，请检查验证码后重试。',
    sessionExpired: '登录已过期，请重新登录',
    invalidCredentials: '邮箱或密码错误',
    weak: '弱',
    medium: '中',
    strong: '强',
    passwordStrength: '密码强度',
    passwordStrengthHint: '建议至少8位，包含大小写、数字和符号',
    passwordMismatch: '两次输入的密码不一致',
    emailRequired: '请输入邮箱',
    passwordRequired: '请输入密码',
    invalidEmail: '邮箱格式不正确',
    passwordTooShort: '密码至少需要6个字符',

    // Landing Page
    features: '功能',
    howItWorks: '如何运作',
    community: '社区',
    language: '语言',
    loggedInAs: '已登录为',
    exitLogin: '退出登录',
    signIn: '登录',
    signUp: '注册',
    registrationClosed: '注册已关闭',
    registrationClosedMessage:
      '平台当前不开放新用户注册，如需访问请联系管理员获取账号。',

    // Hero Section
    githubStarsInDays: '3 天内 2.5K+ GitHub Stars',
    heroTitle1: 'Read the Market.',
    heroTitle2: 'Write the Trade.',
    heroDescription:
      'AUAIEX 是 AI 交易的未来标准——一个开放、社区驱动的代理式交易操作系统。支持 Binance 和 OKX，自托管、多代理竞争，让 AI 为你自动决策、执行和优化交易。',
    poweredBy: '由 Binance 和 OKX 提供支持，Amber.ac 战略投资。',

    // Landing Page CTA
    readyToDefine: '准备好定义 AI 交易的未来吗？',
    startWithCrypto:
      '从加密市场起步，扩展到 TradFi。AUAIEX 是 AgentFi 的基础架构。',
    getStartedNow: '立即开始',
    viewSourceCode: '查看源码',

    // Features Section
    coreFeatures: '核心功能',
    whyChooseAuaiex: '为什么选择 AUAIEX？',
    openCommunityDriven: '开源、透明、社区驱动的 AI 交易操作系统',
    openSourceSelfHosted: '100% 开源与自托管',
    openSourceDesc: '你的框架，你的规则。非黑箱，支持自定义提示词和多模型。',
    openSourceFeatures1: '完全开源代码',
    openSourceFeatures2: '支持自托管部署',
    openSourceFeatures3: '自定义 AI 提示词',
    openSourceFeatures4: '多模型支持（DeepSeek、Qwen）',
    multiAgentCompetition: '多代理智能竞争',
    multiAgentDesc: 'AI 策略在沙盒中高速战斗，最优者生存，实现策略进化。',
    multiAgentFeatures1: '多 AI 代理并行运行',
    multiAgentFeatures2: '策略自动优化',
    multiAgentFeatures3: '沙盒安全测试',
    multiAgentFeatures4: '跨市场策略移植',
    secureReliableTrading: '安全可靠交易',
    secureDesc: '企业级安全保障，完全掌控你的资金和交易策略。',
    secureFeatures1: '本地私钥管理',
    secureFeatures2: 'API 权限精细控制',
    secureFeatures3: '实时风险监控',
    secureFeatures4: '交易日志审计',

    // About Section
    aboutAuaiex: '关于 AUAIEX',
    whatIsAuaiex: '什么是 AUAIEX？',
    auaiexNotAnotherBot: "AUAIEX 不是另一个交易机器人，而是 AI 交易的 'Linux' ——",
    auaiexDescription1: "一个透明、可信任的开源 OS，提供统一的 '决策-风险-执行'",
    auaiexDescription2: '层，支持所有资产类别。',
    auaiexDescription3:
      '从加密市场起步（24/7、高波动性完美测试场），未来扩展到股票、期货、外汇。核心：开放架构、AI',
    auaiexDescription4:
      '达尔文主义（多代理自竞争、策略进化）、CodeFi 飞轮（开发者 PR',
    auaiexDescription5: '贡献获积分奖励）。',
    youFullControl: '你 100% 掌控',
    fullControlDesc: '完全掌控 AI 提示词和资金',
    startupMessages1: '启动自动交易系统...',
    startupMessages2: 'API服务器启动在端口 8080',
    startupMessages3: 'Web 控制台 http://localhost:3000',

    // How It Works Section
    howToStart: '如何开始使用 AUAIEX',
    fourSimpleSteps: '四个简单步骤，开启 AI 自动交易之旅',
    step1Title: '拉取 GitHub 仓库',
    step1Desc:
      'git clone https://github.com/tinkle-community/auaiex 并切换到 dev 分支测试新功能。',
    step2Title: '配置环境',
    step2Desc: '前端设置交易所 API（Binance 和 OKX）、AI 模型和自定义提示词。',
    step3Title: '部署与运行',
    step3Desc:
      '一键 Docker 部署，启动 AI 代理。注意：高风险市场，仅用闲钱测试。',
    step4Title: '优化与贡献',
    step4Desc: '监控交易，提交 PR 改进框架。加入 Telegram 分享策略。',
    importantRiskWarning: '重要风险提示',
    riskWarningText:
      'dev 分支不稳定，勿用无法承受损失的资金。AUAIEX 非托管，无官方策略。交易有风险，投资需谨慎。',

    // Community Section (testimonials are kept as-is since they are quotes)

    // Footer Section
    futureStandardAI: 'AI 交易的未来标准',
    links: '链接',
    resources: '资源',
    documentation: '文档',
    supporters: '支持方',
    strategicInvestment: '(战略投资)',

    // Login Modal
    accessAuaiexPlatform: '访问 AUAIEX 平台',
    loginRegisterPrompt: '请选择登录或注册以访问完整的 AI 交易平台',
    registerNewAccount: '注册新账号',

    // Candidate Coins Warnings
    candidateCoins: '候选币种',
    candidateCoinsZeroWarning: '候选币种数量为 0',
    possibleReasons: '可能原因：',
    coinPoolApiNotConfigured: '币种池API未配置或无法访问（请检查信号源设置）',
    apiConnectionTimeout: 'API连接超时或返回数据为空',
    noCustomCoinsAndApiFailed: '未配置自定义币种且API获取失败',
    solutions: '解决方案：',
    setCustomCoinsInConfig: '在交易员配置中设置自定义币种列表',
    orConfigureCorrectApiUrl: '或者配置正确的币种池API地址',
    orDisableCoinPoolOptions: '或者禁用"使用币种池"和"使用OI Top"选项',
    signalSourceNotConfigured: '信号源未配置',
    signalSourceWarningMessage:
      '您有交易员启用了"使用币种池"或"使用OI Top"，但尚未配置信号源API地址。这将导致候选币种数量为0，交易员无法正常工作。',
    configureSignalSourceNow: '立即配置信号源',

    // FAQ Page
    faqTitle: '常见问题',
    faqSubtitle: '查找关于 AUAIEX 的常见问题解答',
    faqStillHaveQuestions: '还有其他问题？',
    faqContactUs: '加入我们的社区或查看 GitHub 获取更多帮助',

    // FAQ Categories
    faqCategoryBasics: '基础问题',
    faqCategoryContributing: '贡献与任务',
    faqCategorySetup: '安装与配置',
    faqCategoryTrading: '交易问题',
    faqCategoryTechnical: '技术问题',
    faqCategoryAI: 'AI与模型问题',
    faqCategoryData: '数据与隐私',

    // FAQ Questions & Answers - General
    faqWhatIsAUAIEX: 'AUAIEX 是什么？',
    faqWhatIsAUAIEXAnswer:
      'AUAIEX 是一个 AI 驱动的加密货币交易机器人，使用大语言模型（LLM）在期货市场进行交易决策。',

    faqSupportedExchanges: '支持哪些交易所？',
    faqSupportedExchangesAnswer:
      '支持币安合约（Binance Futures）和 OKX Futures。',

    faqIsProfitable: 'AUAIEX 能盈利吗？',
    faqIsProfitableAnswer:
      'AI 交易是实验性的，不保证盈利。请始终用小额资金测试，不要投入超过您承受能力的资金。',

    faqMultipleTraders: '可以同时运行多个交易员吗？',
    faqMultipleTradersAnswer:
      '可以！AUAIEX 支持运行多个交易员，每个可配置不同的 AI 模型和交易策略。',

    // Contributing & Community
    faqGithubProjectsTasks: '如何在 GitHub Projects 中领取任务？',
    faqGithubProjectsTasksAnswer:
      '路线图：https://github.com/orgs/AuaiexAI/projects/3 ｜ 任务看板：https://github.com/orgs/AuaiexAI/projects/5 ｜ 步骤：打开链接 → 按标签筛选（good first issue / help wanted / frontend / backend）→ 阅读描述与验收标准 → 评论“assign me”或自助分配 → Fork 仓库 → 同步你 fork 的 dev 与 upstream/dev → 从你 fork 的 dev 创建特性分支 → 推送到你的 fork → 打开 PR（base：AuaiexAI/auaiex:dev ← compare：你的用户名/auaiex:feat/your-topic）→ 关联 Issue（Closes #123）并选择正确模板。',

    faqContributePR: '如何规范地提交 PR 并参与贡献？',
    faqContributePRAnswer:
      '规范：• 先 Fork；在你的 fork 的 dev 分支上创建特性分支（避免直接向上游 main 提交）• 分支命名：feat/...、fix/...、docs/...；提交信息遵循 Conventional Commits • PR 前运行检查：npm --prefix web run lint && npm --prefix web run build • 涉及 UI 变更请附截图/短视频 • 选择正确 PR 模板（frontend/backend/docs/general）• 从你的 fork 发起到 AuaiexAI/auaiex:dev，并在 PR 中关联 Issue（Closes #123）• 持续 rebase 到 upstream/dev，确保 CI 通过；尽量保持 PR 小而聚焦 • 参考 CONTRIBUTING.md 与 .github/PR_TITLE_GUIDE.md',

    // Setup & Configuration
    faqSystemRequirements: '系统要求是什么？',
    faqSystemRequirementsAnswer:
      '操作系统：Linux、macOS 或 Windows（推荐 Docker）；内存：最低 2GB，推荐 4GB；硬盘：应用 + 日志需要 1GB；网络：稳定的互联网连接。',

    faqNeedCoding: '需要编程经验吗？',
    faqNeedCodingAnswer:
      '不需要！AUAIEX 有 Web 界面进行所有配置。但基础的命令行知识有助于安装和故障排查。',

    faqGetApiKeys: '如何获取 API 密钥？',
    faqGetApiKeysAnswer:
      '币安：账户 → API 管理 → 创建 API → 启用合约。OKX：创建具备合约交易权限的 API Key，并保存 Passphrase。',

    faqUseSubaccount: '应该使用子账户吗？',
    faqUseSubaccountAnswer:
      '推荐：是的，使用专门的子账户运行 AUAIEX 可以更好地隔离风险。但请注意，某些子账户有限制（例如币安子账户最高 5 倍杠杆）。',

    faqDockerDeployment: 'Docker 部署一直失败',
    faqDockerDeploymentAnswer:
      '常见问题：网络连接问题、依赖安装失败、内存不足（需要至少 2C2G）。如果卡在 "go build" 不动，尝试：docker compose down && docker compose build --no-cache && docker compose up -d',

    faqBalanceZero: '账户余额显示为 0',
    faqBalanceZeroAnswer:
      '资金可能在现货账户而非合约账户，或被理财功能锁定。您需要在币安手动将资金划转到合约账户。',

    faqTestnet: '可以使用测试网测试吗？',
    faqTestnetAnswer:
      '暂时不支持测试网。我们建议使用真实交易但小额资金（10-50 USDT）进行测试。',

    // Trading Questions
    faqNoTrades: '为什么我的交易员不开仓？',
    faqNoTradesAnswer:
      '常见原因：AI 根据市场情况决定"等待"；余额或保证金不足；达到持仓上限（默认最多 3 个仓位）；查看故障排查指南了解详细诊断。',

    faqDecisionFrequency: 'AI 多久做一次决策？',
    faqDecisionFrequencyAnswer:
      '可配置！默认是每 3-5 分钟。太频繁 = 过度交易，太慢 = 错过机会。',

    faqCustomStrategy: '可以自定义交易策略吗？',
    faqCustomStrategyAnswer:
      '可以！您可以调整杠杆设置、修改币种选择池、更改决策间隔、自定义系统提示词（高级）。',

    faqMaxPositions: '最多可以同时持有多少个仓位？',
    faqMaxPositionsAnswer:
      '默认：3 个仓位。这是 AI 提示词中的软限制，不是硬编码。',

    faqMarginInsufficient: '保证金不足错误 (code=-2019)',
    faqMarginInsufficientAnswer:
      '常见原因：资金未划转到合约账户；杠杆倍数设置过高（默认 20-50 倍）；已有持仓占用保证金；需要先从现货账户划转 USDT 到合约账户。',

    faqHighFees: '交易手续费太高',
    faqHighFeesAnswer:
      'AUAIEX 默认 3 分钟扫描间隔会导致频繁交易。解决方案：将决策间隔增加到 5-10 分钟；优化系统提示词减少过度交易；调整杠杆降低仓位大小。',

    faqNoTakeProfit: 'AI 不平掉盈利的仓位',
    faqNoTakeProfitAnswer:
      'AI 可能认为趋势会继续。系统目前缺少移动止盈功能。您可以手动平仓或调整系统提示词使其在获利时更保守。',

    // Technical Issues
    faqBinanceApiFailed: '币安 API 调用失败 (code=-2015)',
    faqBinanceApiFailedAnswer:
      '错误："Invalid API-key, IP, or permissions for action"。解决方案：将服务器 IP 添加到币安 API 白名单；检查 API 权限（需要读取 + 合约交易）；确保使用合约 API 而非统一账户 API；VPN IP 可能不稳定。',

    faqBinancePositionMode: '币安持仓模式错误 (code=-4061)',
    faqBinancePositionModeAnswer:
      '错误信息："Order\'s position side does not match user\'s setting"。解决方法：切换为双向持仓模式。登录币安合约 → 点击右上角偏好设置 → 选择持仓模式 → 双向持仓。注意：先平掉所有持仓。',

    faqPortInUse: '后端无法启动 / 端口被占用',
    faqPortInUseAnswer:
      '使用 "lsof -i :8080" 查看占用端口的进程，在 .env 中修改端口：AUAIEX_BACKEND_PORT=8081。',

    faqFrontendLoading: '前端一直显示"加载中..."',
    faqFrontendLoadingAnswer:
      '使用 "curl http://localhost:8080/api/health" 检查后端是否运行。应该返回 {"status":"ok"}。如果不是，查看故障排查指南。',

    faqDatabaseLocked: '数据库锁定错误',
    faqDatabaseLockedAnswer:
      '使用 "docker compose down" 或 "pkill auaiex" 停止所有 AUAIEX 进程，然后使用 "docker compose up -d" 重启。',

    faqAiLearningFailed: 'AI 学习数据加载失败',
    faqAiLearningFailedAnswer:
      '原因：TA-Lib 库未正确安装；历史数据不足（需要完成交易）；环境配置问题。安装 TA-Lib：pip install TA-Lib 或检查系统依赖。',

    faqConfigNotEffective: '配置文件修改不生效',
    faqConfigNotEffectiveAnswer:
      'Docker 需要重新构建："docker compose down && docker compose up -d --build"。PM2 需要重启："pm2 restart all"。检查配置文件格式和路径是否正确。',

    // AI & Model Questions
    faqWhichModels: '支持哪些 AI 模型？',
    faqWhichModelsAnswer:
      'DeepSeek（推荐性价比）、Qwen（阿里云通义千问）、自定义 OpenAI 兼容 API（可用于 OpenAI、通过代理的 Claude 或其他提供商）。',

    faqApiCosts: 'API 调用成本是多少？',
    faqApiCostsAnswer:
      '取决于您的模型和决策频率：DeepSeek：每天约 $0.10-0.50（1 个交易员，5 分钟间隔）；Qwen：每天约 $0.20-0.80；自定义 API（例如 OpenAI GPT-4）：每天约 $2-5。基于典型使用的估算。',

    faqMultipleModels: '可以使用多个 AI 模型吗？',
    faqMultipleModelsAnswer:
      '可以！每个交易员可以使用不同的 AI 模型。您甚至可以 A/B 测试不同模型。',

    faqAiLearning: 'AI 会从错误中学习吗？',
    faqAiLearningAnswer:
      '会的，在一定程度上。AUAIEX 在每次决策提示中提供历史表现反馈，允许 AI 调整策略。',

    faqOnlyShort: 'AI 只开空单，不开多单',
    faqOnlyShortAnswer:
      '默认系统提示词包含"不要有做多偏见！做空是你的核心工具之一"，可能导致此问题。还受 4 小时周期数据和模型训练偏向性影响。您可以修改系统提示词使其更平衡。',

    faqModelSelection: '应该使用哪个 DeepSeek 版本？',
    faqModelSelectionAnswer:
      '推荐使用 DeepSeek V3 以获得最佳性能。备选：DeepSeek R1（推理模型，较慢但逻辑更好）、SiliconFlow 的 DeepSeek（备用 API 提供商）。大多数用户反馈 V3 效果良好。',

    // Data & Privacy
    faqDataStorage: '我的数据存储在哪里？',
    faqDataStorageAnswer:
      '所有数据都本地存储在您的机器上，使用 SQLite 数据库：data.db（所有配置和交易历史）、decision_logs/（AI 决策记录）。',

    faqApiKeySecurity: 'API 密钥安全吗？',
    faqApiKeySecurityAnswer:
      'API 密钥存储在本地数据库中。永远不要分享您的数据库或 .env 文件。我们建议使用带 IP 白名单限制的 API 密钥。',

    faqExportHistory: '可以导出交易历史吗？',
    faqExportHistoryAnswer:
      '可以！交易数据是 SQLite 格式。您可以直接查询：sqlite3 trading.db "SELECT * FROM trades;"',

    faqGetHelp: '在哪里可以获得帮助？',
    faqGetHelpAnswer:
      '查看 GitHub Discussions、加入 Telegram 社区或在 GitHub 上提出 issue。',

    // Web Crypto Environment Check
    environmentCheck: {
      button: '一键检测环境',
      checking: '正在检测...',
      description: '系统将自动检测当前浏览器是否允许使用 Web Crypto。',
      secureTitle: '环境安全，已启用 Web Crypto',
      secureDesc: '页面处于安全上下文，可继续输入敏感信息并使用加密传输。',
      insecureTitle: '检测到非安全环境',
      insecureDesc:
        '当前访问未通过 HTTPS 或可信 localhost，浏览器会阻止 Web Crypto 调用。',
      tipsTitle: '修改建议：',
      tipHTTPS:
        '通过 HTTPS 访问（即使是 IP 也需证书），或部署到支持 TLS 的域名。',
      tipLocalhost: '开发阶段请使用 http://localhost 或 127.0.0.1。',
      tipIframe:
        '避免把应用嵌入在不安全的 HTTP iframe 或会降级协议的反向代理中。',
      unsupportedTitle: '浏览器未提供 Web Crypto',
      unsupportedDesc:
        '请通过 HTTPS 或本机 localhost 访问 AUAIEX，并避免嵌入不安全 iframe/反向代理，以符合浏览器的 Web Crypto 规则。',
      summary: '当前来源：{origin} · 协议：{protocol}',
      disabledTitle: '传输加密已禁用',
      disabledDesc:
        '服务端传输加密已关闭，API 密钥将以明文传输。如需增强安全性，请设置 TRANSPORT_ENCRYPTION=true。',
    },

    environmentSteps: {
      checkTitle: '1. 环境检测',
      selectTitle: '2. 选择交易所',
    },

    // Two-Stage Key Modal
    twoStageKey: {
      title: '两阶段私钥输入',
      stage1Description: '请输入私钥的前 {length} 位字符',
      stage2Description: '请输入私钥的后 {length} 位字符',
      stage1InputLabel: '第一部分',
      stage2InputLabel: '第二部分',
      characters: '位字符',
      processing: '处理中...',
      nextButton: '下一步',
      cancelButton: '取消',
      backButton: '返回',
      encryptButton: '加密并提交',
      obfuscationCopied: '混淆数据已复制到剪贴板',
      obfuscationInstruction: '请粘贴其他内容清空剪贴板，然后继续',
      obfuscationManual: '需要手动混淆',
    },

    // Error Messages
    errors: {
      privatekeyIncomplete: '请输入至少 {expected} 位字符',
      privatekeyInvalidFormat: '私钥格式无效（应为64位十六进制字符）',
      privatekeyObfuscationFailed: '剪贴板混淆失败',
    },
  },
}

export function t(
  key: string,
  lang: Language,
  params?: Record<string, string | number>
): string {
  // Handle nested keys like 'twoStageKey.title'
  const keys = key.split('.')
  let value: any = translations[lang]

  for (const k of keys) {
    value = value?.[k]
  }

  let text = typeof value === 'string' ? value : key

  // Replace parameters like {count}, {gap}, etc.
  if (params) {
    Object.entries(params).forEach(([param, value]) => {
      text = text.replace(`{${param}}`, String(value))
    })
  }

  return text
}
