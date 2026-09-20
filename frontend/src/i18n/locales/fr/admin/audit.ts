export default {
  audit: {
    title: 'Journaux d’audit',
    description: 'Enregistre les opérations du plan de gestion des administrateurs et des utilisateurs. Les identifiants d’en-tête ne conservent que leurs premiers et derniers caractères, et les corps de requête sont expurgés. Les entrées ne peuvent pas être supprimées individuellement ; le nettoyage complet exige une vérification à deux facteurs.',
    clearAll: 'Tout effacer',
    empty: 'Aucun journal d’audit pour le moment',
    loadFailed: 'Échec du chargement des journaux d’audit',
    tabs: {
      operations: 'Journaux d’audit',
      tickets: 'Journaux de tickets'
    },
    ticketLogs: {
      hint: 'Lecture des enregistrements de collecte de tickets en mémoire pour ce processus. Ils ne sont pas stockés en base et disparaissent après un redémarrage ou un effacement. Au plus les {count} plus récents sont conservés.',
      empty: 'Aucun journal de tickets pour le moment',
      loadFailed: 'Échec du chargement des journaux de tickets',
      clear: 'Effacer',
      clearConfirmTitle: 'Effacer les journaux de tickets',
      clearConfirmMessage: 'Cela supprime uniquement les enregistrements en mémoire de ce processus. Aucune vérification à deux facteurs n’est requise. Continuer ?',
      clearSuccess: 'Journaux de tickets effacés',
      account: 'Compte',
      model: 'Modèle'
    },
    filters: {
      all: 'Tous',
      q: 'Mot-clé',
      qPlaceholder: 'Chemin / action / e-mail de l’acteur',
      actorEmail: 'E-mail de l’acteur',
      action: 'Action',
      clientIp: 'IP client',
      method: 'Méthode',
      authMethod: 'Méthode d’authentification',
      result: 'Résultat',
      resultSuccess: 'Succès',
      resultFailure: 'Échec',
      startTime: 'Heure de début',
      endTime: 'Heure de fin'
    },
    columns: {
      time: 'Heure',
      actor: 'Acteur',
      action: 'Action',
      method: 'Méthode',
      result: 'Résultat',
      clientIp: 'IP client',
      detail: 'Détail'
    },
    detail: {
      title: 'Détail du journal d’audit',
      actorRole: 'Rôle',
      methodPath: 'Méthode / Chemin',
      latency: 'Latence',
      requestId: 'ID de requête',
      credential: 'Identifiant (masqué)',
      userAgent: 'User-Agent',
      requestBody: 'Corps de la requête (expurgé)',
      extra: 'Supplément'
    },
    clearConfirm: {
      title: 'Effacer tous les journaux d’audit',
      message: 'Cette action supprime définitivement tous les journaux d’audit et ne peut pas être annulée. L’action d’effacement elle-même est enregistrée. Continuer ?',
      totpTitle: 'Saisir le code à deux facteurs',
      totpHint: 'L’effacement des journaux d’audit exige une vérification TOTP récente.',
      success: '{count} journal(aux) d’audit effacé(s)',
      failed: 'Échec de l’effacement des journaux d’audit'
    }
  }
}
