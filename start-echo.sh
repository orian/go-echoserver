#!/bin/bash

if [ ! -z "$DB_SSL_CERT" ]
then
  echo "change permission for: \$DB_SSL_CERT"
  NEW=/tmp/cert
  cp $DB_SSL_CERT $NEW
  chmod 0600 $NEW
  export DB_SSL_CERT=$NEW
else
  echo "\$DB_SSL_CERT is empty"
fi

if [ ! -z "$DB_SSL_KEY" ]
then
  echo "change permission for: \$DB_SSL_KEY"
  NEW=/tmp/key
  cp $DB_SSL_KEY $NEW
  chmod 0600 $NEW
  export DB_SSL_KEY=$NEW
else
  echo "\$DB_SSL_KEY is empty"
fi

if [ ! -z "$DB_SSL_ROOT_CERT" ]
then
  echo "change permission for: \$DB_SSL_ROOT_CERT"
  NEW=/tmp/root-cert
  cp $DB_SSL_ROOT_CERT $NEW
  chmod 0600 $NEW
  export DB_SSL_ROOT_CERT=$NEW
else
  echo "\$DB_SSL_ROOT_CERT is empty"
fi

/app/echoserver
