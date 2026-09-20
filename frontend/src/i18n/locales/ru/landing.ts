export default {
  batchImageGuide: {
    title: 'Пакетная генерация изображений',
    description: 'Отправьте несколько промптов в одном задании и скачайте сгенерированные изображения по завершении'
  },
  // Home Page
  home: {
    viewOnGithub: 'Смотреть на GitHub',
    viewDocs: 'Смотреть документацию',
    docs: 'Документация',
    switchToLight: 'Переключить на светлую тему',
    switchToDark: 'Переключить на тёмную тему',
    dashboard: 'Панель управления',
    login: 'Вход',
    getStarted: 'Начать',
    goToDashboard: 'Перейти в панель управления',
    // User-focused value proposition
    heroSubtitle: 'Один ключ — все модели ИИ',
    heroDescription: 'Не нужно управлять несколькими подписками. Доступ к Claude, GPT, Gemini и другим моделям с одной API Key',
    tags: {
      subscriptionToApi: 'Подписка в API',
      stickySession: 'Сохранение сессии',
      realtimeBilling: 'Оплата по факту'
    },
    // Pain points section
    painPoints: {
      title: 'Знакомо?',
      items: {
        expensive: {
          title: 'Высокая стоимость подписок',
          desc: 'Оплата нескольких подписок на ИИ, которые складываются каждый месяц'
        },
        complex: {
          title: 'Хаос в аккаунтах',
          desc: 'Управление разрозненными аккаунтами и API Keys на разных платформах'
        },
        unstable: {
          title: 'Перебои в работе сервиса',
          desc: 'Отдельные аккаунты упираются в лимиты и срывают ваш рабочий процесс'
        },
        noControl: {
          title: 'Нет контроля использования',
          desc: 'Невозможно отследить, куда уходят деньги, и ограничить использование участниками команды'
        }
      }
    },
    // Solutions section
    solutions: {
      title: 'Мы решаем эти проблемы',
      subtitle: 'Три простых шага к спокойному доступу к ИИ'
    },
    features: {
      unifiedGateway: 'Доступ в один клик',
      unifiedGatewayDesc: 'Получите одну API Key для вызова всех подключённых моделей ИИ. Отдельные заявки не нужны.',
      multiAccount: 'Всегда надёжно',
      multiAccountDesc: 'Умная маршрутизация между несколькими вышестоящими аккаунтами с автоматическим переключением. Забудьте об ошибках.',
      balanceQuota: 'Платите за фактическое использование',
      balanceQuotaDesc: 'Тарификация по использованию с лимитами квот. Полная видимость потребления команды.'
    },
    // Comparison section
    comparison: {
      title: 'Почему выбирают нас?',
      headers: {
        feature: 'Сравнение',
        official: 'Официальные подписки',
        us: 'Наша платформа'
      },
      items: {
        pricing: {
          feature: 'Тарификация',
          official: 'Фиксированная ежемесячная плата, даже без использования',
          us: 'Платите только за то, что используете'
        },
        models: {
          feature: 'Выбор моделей',
          official: 'Только один поставщик',
          us: 'Свободно переключайтесь между моделями'
        },
        management: {
          feature: 'Управление аккаунтами',
          official: 'Каждый сервис управляется отдельно',
          us: 'Единый ключ, одна панель'
        },
        stability: {
          feature: 'Стабильность',
          official: 'Лимиты одного аккаунта',
          us: 'Пул аккаунтов, автопереключение'
        },
        control: {
          feature: 'Контроль использования',
          official: 'Недоступно',
          us: 'Квоты и подробная аналитика'
        }
      }
    },
    providers: {
      title: 'Поддерживаемые модели ИИ',
      description: 'Один API, много вариантов',
      supported: 'Поддерживается',
      soon: 'Скоро',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'Ещё'
    },
    // CTA section
    cta: {
      title: 'Готовы начать?',
      description: 'Зарегистрируйтесь сейчас и получите бесплатные пробные кредиты, чтобы оценить удобный доступ к ИИ',
      button: 'Бесплатная регистрация'
    },
    footer: {
      allRightsReserved: 'Все права защищены.'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'Использование API Key',
    subtitle: 'Введите вашу API Key, чтобы просмотреть расходы и статус использования в реальном времени',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: 'Запросить',
    querying: 'Запрос...',
    privacyNote: 'Ваш API Key обрабатывается локально в браузере и не будет сохранён',
    dateRange: 'Диапазон дат:',
    dateRangeToday: 'Сегодня',
    dateRange7d: '7 дней',
    dateRange30d: '30 дней',
    dateRange90d: '90 дней',
    dateRangeCustom: 'Свой',
    apply: 'Применить',
    used: 'Использовано',
    detailInfo: 'Подробная информация',
    tokenStats: 'Статистика токенов',
    dailyDetail: 'По дням',
    modelStats: 'Статистика использования моделей',
    // Table headers
    date: 'Дата',
    model: 'Модель',
    requests: 'Запросы',
    inputTokens: 'Входные токены',
    outputTokens: 'Выходные токены',
    cacheCreationTokens: 'Создание кэша',
    cacheReadTokens: 'Чтение кэша',
    cacheWriteTokens: 'Запись кэша',
    totalTokens: 'Всего токенов',
    cost: 'Стоимость',
    // Status
    quotaMode: 'Режим квоты ключа',
    walletBalance: 'Баланс кошелька',
    // Ring card titles
    totalQuota: 'Общая квота',
    limit5h: 'Лимит за 5 часов',
    limitDaily: 'Дневной лимит',
    limit7d: 'Лимит за 7 дней',
    limitWeekly: 'Недельный лимит',
    limitMonthly: 'Месячный лимит',
    // Detail rows
    remainingQuota: 'Оставшаяся квота',
    expiresAt: 'Истекает',
    todayExpires: '(истекает сегодня)',
    daysLeft: '({days} дн.)',
    usedQuota: 'Использованная квота',
    windowDay: 'Д',
    windowWeek: 'Н',
    windowMonth: 'М',
    resetNow: 'Скоро сброс',
    subscriptionType: 'Тип подписки',
    billingType: 'Тип тарификации',
    subscriptionExpires: 'Окончание подписки',
    // Usage stat cells
    todayRequests: 'Запросы сегодня',
    todayInputTokens: 'Вход сегодня',
    todayOutputTokens: 'Выход сегодня',
    todayTokens: 'Токены сегодня',
    todayCacheCreation: 'Создание кэша сегодня',
    todayCacheRead: 'Чтение кэша сегодня',
    todayCost: 'Стоимость сегодня',
    rpmTpm: 'RPM / TPM',
    totalRequests: 'Всего запросов',
    totalInputTokens: 'Всего вход',
    totalOutputTokens: 'Всего выход',
    totalTokensLabel: 'Всего токенов',
    totalCacheCreation: 'Всего создание кэша',
    totalCacheRead: 'Всего чтение кэша',
    totalCost: 'Общая стоимость',
    avgDuration: 'Средняя длительность',
    // Messages
    enterApiKey: 'Пожалуйста, введите API Key',
    querySuccess: 'Запрос выполнен',
    queryFailed: 'Запрос не выполнен',
    queryFailedRetry: 'Запрос не выполнен, повторите попытку позже',
    noDailyUsage: 'Нет данных об использовании по дням',
  },

  // Setup Wizard
  setup: {
    title: 'Установка Sub2API',
    description: 'Настройте ваш экземпляр Sub2API',
    database: {
      title: 'Конфигурация базы данных',
      description: 'Подключитесь к вашей базе данных PostgreSQL',
      host: 'Хост',
      port: 'Порт',
      username: 'Имя пользователя',
      password: 'Пароль',
      databaseName: 'Имя базы данных',
      sslMode: 'Режим SSL',
      passwordPlaceholder: 'Пароль',
      ssl: {
        disable: 'Отключить',
        require: 'Требовать',
        verifyCa: 'Проверять CA',
        verifyFull: 'Полная проверка'
      }
    },
    redis: {
      title: 'Конфигурация Redis',
      description: 'Подключитесь к вашему серверу Redis',
      host: 'Хост',
      port: 'Порт',
      username: 'Имя пользователя (необязательно)',
      password: 'Пароль (необязательно)',
      database: 'База данных',
      usernamePlaceholder: 'Оставьте пустым для пользователя по умолчанию',
      passwordPlaceholder: 'Пароль',
      enableTls: 'Включить TLS',
      enableTlsHint: 'Использовать TLS при подключении к Redis (публичные сертификаты CA)'
    },
    admin: {
      title: 'Учётная запись администратора',
      description: 'Создайте учётную запись администратора',
      email: 'Электронная почта',
      password: 'Пароль',
      confirmPassword: 'Подтвердите пароль',
      passwordPlaceholder: 'Не менее 8 символов',
      confirmPasswordPlaceholder: 'Подтвердите пароль',
      passwordMismatch: 'Пароли не совпадают'
    },
    ready: {
      title: 'Готово к установке',
      description: 'Проверьте конфигурацию и завершите установку',
      database: 'База данных',
      redis: 'Redis',
      adminEmail: 'Эл. почта администратора'
    },
    status: {
      testing: 'Проверка...',
      success: 'Подключение успешно',
      testConnection: 'Проверить подключение',
      installing: 'Установка...',
      completeInstallation: 'Завершить установку',
      completed: 'Установка завершена!',
      redirecting: 'Перенаправление на страницу входа...',
      restarting: 'Служба перезапускается, пожалуйста, подождите...',
      timeout: 'Перезапуск службы занимает больше времени, чем ожидалось. Обновите страницу вручную.'
    }
  },

  // Common
}
