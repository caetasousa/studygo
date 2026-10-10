-- O token do Claude que quem estuda guarda em Configurações, para o
-- processador de mapas usar a assinatura dela. Guarda-se CIFRADO (AES-GCM, com a
-- chave derivada do segredo do JWT): um vazamento do banco não entrega o acesso
-- à assinatura. NULL é sem token — o processador usa a conexão da conta.
--
-- A 000018 não existe: saiu antes de publicar, mas está aplicada em bancos
-- locais, e reusar o número faria esta ser pulada neles.
ALTER TABLE usuarios ADD COLUMN token_claude bytea;
