#!/bin/bash

# Repository base path
REPO="quay.io/metoro/metoro-demo-applications"
VERSION="1.0.1"

# Check if buildx is available
if ! docker buildx version > /dev/null 2>&1; then
  echo "Docker buildx is required for multi-architecture builds."
  echo "Please install Docker buildx: https://docs.docker.com/buildx/working-with-buildx/"
  exit 1
fi

# Ensure builder instance with multi-platform support exists
if ! docker buildx inspect multiarch-builder > /dev/null 2>&1; then
  echo "Creating multi-architecture builder instance..."
  docker buildx create --name multiarch-builder --platform linux/amd64,linux/arm64 --use
else
  docker buildx use multiarch-builder
fi

# Make sure the builder is running
docker buildx inspect --bootstrap

echo "Building and pushing multi-architecture images to $REPO..."

# Function to build and push a service
build_and_push() {
  SERVICE=$1
  TAG="$REPO:$SERVICE-$VERSION"
  LATEST_TAG="$REPO:$SERVICE-latest"
  
  echo "===================================="
  echo "Building $SERVICE service for amd64 and arm64..."
  echo "===================================="
  
  # Build the image for multiple architectures and push
  docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --tag $TAG \
    --tag $LATEST_TAG \
    --push \
    ./$SERVICE

  if [ $? -eq 0 ]; then
    echo "✅ Successfully built and pushed $SERVICE service for multiple architectures"
    echo "  - $TAG"
    echo "  - $LATEST_TAG"
  else
    echo "❌ Failed to build or push $SERVICE service"
    exit 1
  fi
}

# Build and push each service
build_and_push "gateway"
build_and_push "product-catalog"
build_and_push "currency-service"
build_and_push "ad-service"
build_and_push "checkout-service"
build_and_push "load-generator"

echo "===================================="
echo "All services built and pushed successfully!"
echo "===================================="
echo "Multi-architecture images available at:"
echo "  $REPO:gateway-$VERSION (and :gateway-latest)"
echo "  $REPO:product-catalog-$VERSION (and :product-catalog-latest)"
echo "  $REPO:currency-service-$VERSION (and :currency-service-latest)"
echo "  $REPO:ad-service-$VERSION (and :ad-service-latest)"
echo "  $REPO:checkout-service-$VERSION (and :checkout-service-latest)"
echo "  $REPO:load-generator-$VERSION (and :load-generator-latest)"
echo ""
echo "Images support both amd64 and arm64 architectures."
echo ""
echo "To update Helm chart to use version $VERSION:"
echo "  helm upgrade microservice-demo ./helm-chart \\"
echo "    --set global.tag=$VERSION \\"
echo "    --set gateway.image.tag=gateway-$VERSION \\"
echo "    --set productCatalog.image.tag=product-catalog-$VERSION \\"
echo "    --set currencyService.image.tag=currency-service-$VERSION \\"
echo "    --set adService.image.tag=ad-service-$VERSION \\"
echo "    --set checkoutService.image.tag=checkout-service-$VERSION \\"
echo "    --set loadGenerator.image.tag=load-generator-$VERSION" 