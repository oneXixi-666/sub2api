export default {
  upstreamFunds: {
    title: '上游资金中心',
    description: '按真实上游钱包归并账号，跟踪采购余额、实际成本和资金储备。',
    createWallet: '新增上游钱包',
    editWallet: '编辑上游钱包',
    recordBalance: '录入余额',
    recharge: '充值',
    adapterPending: '待接入适配器',
    phaseOne: '监控阶段',
    adapterPendingHint: '当前仅支持资金监控和手工余额快照；自动充值需接入真实上游适配器后启用。',
    searchPlaceholder: '搜索钱包或上游标识',
    summary: {
      wallets: '上游钱包',
      enabled: '{count} 个启用',
      todayCost: '今日实际上游成本',
      balance: '可查询余额',
      attention: '需要关注',
      costCurrency: '成本统一按 USD 统计',
      cost24h: '近 24 小时成本'
    },
    wallet: {
      balance: '当前余额',
      updated: '更新于 {time}',
      neverUpdated: '尚未录入',
      runway: '预计可用',
      runwayDays: '{days} 天',
      runwayUnknown: '待计算',
      runwayCurrencyMismatch: '余额币种与成本币种不同，暂不计算可用天数',
      runwayNoCost: '近 7 天暂无实际上游成本，暂不计算可用天数',
      alertLine: '告警线 {days} 天',
      targetLine: '目标 {days} 天',
      cost1h: '近 1 小时',
      costToday: '今日',
      cost24h: '近 24 小时',
      cost7d: '近 7 天',
      suggestedTopUp: '建议补充 {amount}',
      healthyReserve: '当前储备达到目标',
      accounts: '关联账号',
      configuredGroups: '配置分组',
      actualGroups: '近 7 天实际分组',
      noAccounts: '尚未关联账号',
      noGroups: '暂无分组',
      disabled: '已停用',
      attention: '资金告警'
    },
    tier: {
      primary: '主力',
      hot_backup: '热备',
      cold_backup: '冷备'
    },
    mode: {
      direct: '直充',
      product: '商品/卡券',
      manual: '人工处理'
    },
    form: {
      name: '钱包名称',
      namePlaceholder: '例如：主力 OpenAI 钱包',
      provider: '上游标识',
      providerPlaceholder: '例如：provider_a',
      currency: '余额币种',
      rechargeMode: '充值模式',
      tier: '运营层级',
      alertDays: '余额告警天数',
      targetDays: '目标储备天数',
      enabled: '启用资金监控',
      accounts: '共用这个钱包的上游账号',
      accountSearch: '搜索账号名称或平台',
      accountOwned: '已归属：{wallet}',
      selectedAccounts: '已选择 {count} 个账号',
      balance: '最新余额',
      balanceHint: '本次录入会新增一条不可覆盖的余额快照。'
    },
    empty: {
      title: '还没有上游钱包',
      description: '先把共用同一份真实余额的上游账号归到一个钱包。'
    },
    messages: {
      loadFailed: '加载上游资金数据失败',
      saveFailed: '保存上游钱包失败',
      balanceFailed: '录入余额失败',
      created: '上游钱包已创建',
      updated: '上游钱包已更新',
      balanceRecorded: '余额快照已记录',
      invalidReserve: '目标储备天数不能小于告警天数'
    }
  }
}
