C'est un repository pour investiguer et construire un outil CLI et MCP pour contrôler le site https://www.local.bio. Il permettra de se connecter, de choisir un point de retrait, de rechercher des produits, de les
ajouter au panier, de consulter le panier, de consulter les dernières commandes.


Le tout sera écrit en go, et on utilisera devenv pour wrap tout ça pour le dev. 
Il y aura aussi une image Docker qui se chargera de lancer le serveur MCP HTTP. Le MCP sera aussi accessible en stdio.
Note aussi que tout l'environnement de dev doit utiliser devenv, avec des scripts adaptés et un direnv fonctionnel.

Au niveau CLI, voici les commandes disponibles : 

  login                       Log in with your local.bio account
  logout                      Log out and clear stored tokens
  info                        Show info about your account
  store set <ref>             Select your store
  store search <query>        Search stores by city or postal code
  orders                      List previous orders
  orders <number>             Show order detail with articles
  search <query>              Search for products
  basket get                  Show current basket contents
  basket add <ean> [qty]      Add a product to your basket (default qty: 1)
  basket remove <ean> [qty]   Remove a product (default: remove all)
  mcp                         Start MCP server (stdio transport)
  mcp http [addr]             Start MCP server (Streamable HTTP, default :8080)

Il y a un Chrome accessible via CDP uniquement pour la partie dev. La CLI ne doit en aucun cas l'utiliser autrement que pour explorer la site.
Pour la CLI, ajoute une option "--format json" qui permet de formater ça en JSON. Pour les MCP, utilise un format approprié pour les LLM.
Il faudra écrire des Workflow pour Github Action de manière similaire à https://github.com/NicolasGuilloux/intermarche-mcp.
